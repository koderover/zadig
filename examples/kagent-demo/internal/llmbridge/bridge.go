package llmbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"hello-agent/internal/llmconfig"
)

const (
	ProviderOpenAI    = llmconfig.ProviderOpenAI
	ProviderAnthropic = llmconfig.ProviderAnthropic
)

type Bridge struct {
	cfg    llmconfig.Config
	client *http.Client
}

func New(cfg llmconfig.Config, client *http.Client) *Bridge {
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &Bridge{cfg: cfg, client: client}
}

func (b *Bridge) Name() string {
	return b.cfg.Model
}

func (b *Bridge) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return b.generateStream(ctx, req)
	}
	return func(yield func(*model.LLMResponse, error) bool) {
		resp, err := b.generate(ctx, req)
		yield(resp, err)
	}
}

func (b *Bridge) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	switch b.cfg.Provider {
	case llmconfig.ProviderOpenAI:
		return b.generateOpenAIStream(ctx, req)
	case llmconfig.ProviderAnthropic:
		return b.generateAnthropicStream(ctx, req)
	default:
		return func(yield func(*model.LLMResponse, error) bool) {
			yield(nil, fmt.Errorf("unsupported provider %q", b.cfg.Provider))
		}
	}
}

func (b *Bridge) generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	switch b.cfg.Provider {
	case llmconfig.ProviderOpenAI:
		return b.generateOpenAI(ctx, req)
	case llmconfig.ProviderAnthropic:
		return b.generateAnthropic(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider %q", b.cfg.Provider)
	}
}

func (b *Bridge) generateOpenAI(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	payload := b.openAIPayload(req, false)

	var out openAIResponse
	if err := b.postJSON(ctx, b.cfg.BaseURL+"/chat/completions", payload, openAIHeaders(b.cfg.APIKey), &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("openai response contained no choices")
	}
	return openAIChoiceToLLM(out.Choices[0]), nil
}

func (b *Bridge) generateOpenAIStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		body, err := b.postStream(ctx, b.cfg.BaseURL+"/chat/completions", b.openAIPayload(req, true), openAIHeaders(b.cfg.APIKey))
		if err != nil {
			yield(nil, err)
			return
		}
		defer body.Close()

		var fullText strings.Builder
		toolCalls := map[int]*streamingToolCall{}
		if err := readSSE(body, func(data string) bool {
			if data == "[DONE]" {
				return false
			}
			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return yield(nil, fmt.Errorf("decode openai stream chunk: %w", err))
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					fullText.WriteString(choice.Delta.Content)
					if !yield(textResponse(choice.Delta.Content, true), nil) {
						return false
					}
				}
				for _, delta := range choice.Delta.ToolCalls {
					call := toolCalls[delta.Index]
					if call == nil {
						call = &streamingToolCall{}
						toolCalls[delta.Index] = call
					}
					if delta.ID != "" {
						call.id = delta.ID
					}
					if delta.Function.Name != "" {
						call.name = delta.Function.Name
					}
					call.arguments.WriteString(delta.Function.Arguments)
				}
			}
			return true
		}); err != nil {
			yield(nil, err)
			return
		}

		if len(toolCalls) > 0 {
			yield(toolCallResponse(orderedToolCalls(toolCalls)), nil)
			return
		}
		if fullText.Len() > 0 {
			yield(textResponse(fullText.String(), false), nil)
		}
	}
}

func (b *Bridge) generateAnthropic(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	payload := b.anthropicPayload(req, false)

	var out anthropicResponse
	if err := b.postJSON(ctx, b.cfg.BaseURL+"/messages", payload, anthropicHeaders(b.cfg), &out); err != nil {
		return nil, err
	}
	return anthropicToLLM(out), nil
}

func (b *Bridge) generateAnthropicStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		body, err := b.postStream(ctx, b.cfg.BaseURL+"/messages", b.anthropicPayload(req, true), anthropicHeaders(b.cfg))
		if err != nil {
			yield(nil, err)
			return
		}
		defer body.Close()

		var fullText strings.Builder
		toolCalls := map[int]*streamingToolCall{}
		if err := readSSE(body, func(data string) bool {
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return yield(nil, fmt.Errorf("decode anthropic stream event: %w", err))
			}
			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" {
					toolCalls[event.Index] = &streamingToolCall{
						id:   event.ContentBlock.ID,
						name: event.ContentBlock.Name,
					}
				}
			case "content_block_delta":
				switch event.Delta.Type {
				case "text_delta":
					fullText.WriteString(event.Delta.Text)
					if !yield(textResponse(event.Delta.Text, true), nil) {
						return false
					}
				case "input_json_delta":
					call := toolCalls[event.Index]
					if call == nil {
						call = &streamingToolCall{}
						toolCalls[event.Index] = call
					}
					call.arguments.WriteString(event.Delta.PartialJSON)
				}
			case "message_stop":
				return false
			}
			return true
		}); err != nil {
			yield(nil, err)
			return
		}

		if len(toolCalls) > 0 {
			yield(toolCallResponse(orderedToolCalls(toolCalls)), nil)
			return
		}
		if fullText.Len() > 0 {
			yield(textResponse(fullText.String(), false), nil)
		}
	}
}

func (b *Bridge) openAIPayload(req *model.LLMRequest, stream bool) map[string]any {
	payload := map[string]any{
		"model":    b.modelName(req),
		"messages": openAIMessages(req),
	}
	if stream {
		payload["stream"] = true
	}
	if tools := openAITools(req); len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}
	if maxTokens := maxOutputTokens(req); maxTokens > 0 {
		payload["max_tokens"] = maxTokens
	}
	return payload
}

func (b *Bridge) anthropicPayload(req *model.LLMRequest, stream bool) map[string]any {
	system, messages := anthropicMessages(req)
	payload := map[string]any{
		"model":      b.modelName(req),
		"max_tokens": max(maxOutputTokens(req), 1024),
		"messages":   messages,
	}
	if stream {
		payload["stream"] = true
	}
	if system != "" {
		payload["system"] = system
	}
	if tools := anthropicTools(req); len(tools) > 0 {
		payload["tools"] = tools
	}
	return payload
}

func (b *Bridge) modelName(req *model.LLMRequest) string {
	if req != nil && strings.TrimSpace(req.Model) != "" {
		return strings.TrimSpace(req.Model)
	}
	return b.cfg.Model
}

func (b *Bridge) postStream(ctx context.Context, url string, payload any, headers map[string]string) (io.ReadCloser, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call model: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return resp.Body, nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read model response: %w", err)
	}
	return nil, fmt.Errorf("model returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
}

func (b *Bridge) postJSON(ctx context.Context, url string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call model: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read model response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("model returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode model response: %w", err)
	}
	return nil
}

func openAIHeaders(apiKey string) map[string]string {
	return map[string]string{"authorization": "Bearer " + apiKey}
}

func anthropicHeaders(cfg llmconfig.Config) map[string]string {
	return map[string]string{
		"x-api-key": cfg.APIKey,
	}
}

func maxOutputTokens(req *model.LLMRequest) int32 {
	if req == nil || req.Config == nil {
		return 0
	}
	return req.Config.MaxOutputTokens
}

func openAIMessages(req *model.LLMRequest) []map[string]any {
	if req == nil {
		return nil
	}
	var messages []map[string]any
	if sys := contentText(systemInstruction(req)); sys != "" {
		messages = append(messages, map[string]any{"role": "system", "content": sys})
	}
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		messages = append(messages, openAIContentMessages(content)...)
	}
	return messages
}

func openAIContentMessages(content *genai.Content) []map[string]any {
	role := "user"
	if content.Role == genai.RoleModel {
		role = "assistant"
	}

	var textParts []string
	var toolCalls []map[string]any
	var messages []map[string]any
	for _, part := range content.Parts {
		switch {
		case part == nil:
		case part.Text != "":
			textParts = append(textParts, part.Text)
		case part.FunctionCall != nil:
			args, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, map[string]any{
				"id":   fallbackID(part.FunctionCall.ID, part.FunctionCall.Name),
				"type": "function",
				"function": map[string]any{
					"name":      part.FunctionCall.Name,
					"arguments": string(args),
				},
			})
		case part.FunctionResponse != nil:
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": fallbackID(part.FunctionResponse.ID, part.FunctionResponse.Name),
				"name":         part.FunctionResponse.Name,
				"content":      mustJSON(part.FunctionResponse.Response),
			})
		}
	}
	if len(textParts) > 0 || len(toolCalls) > 0 {
		msg := map[string]any{"role": role, "content": strings.Join(textParts, "\n")}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		messages = append([]map[string]any{msg}, messages...)
	}
	return messages
}

func openAITools(req *model.LLMRequest) []map[string]any {
	var tools []map[string]any
	for _, decl := range functionDeclarations(req) {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        decl.Name,
				"description": decl.Description,
				"parameters":  schemaObject(decl.ParametersJsonSchema, decl.Parameters),
			},
		})
	}
	return tools
}

func openAIChoiceToLLM(choice openAIChoice) *model.LLMResponse {
	var parts []*genai.Part
	if choice.Message.Content != "" {
		parts = append(parts, genai.NewPartFromText(choice.Message.Content))
	}
	for _, call := range choice.Message.ToolCalls {
		args := map[string]any{}
		_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{
			ID:   call.ID,
			Name: call.Function.Name,
			Args: args,
		}})
	}
	return &model.LLMResponse{Content: genai.NewContentFromParts(parts, genai.RoleModel)}
}

func anthropicMessages(req *model.LLMRequest) (string, []map[string]any) {
	if req == nil {
		return "", nil
	}
	system := contentText(systemInstruction(req))
	var messages []map[string]any
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		role := "user"
		if content.Role == genai.RoleModel {
			role = "assistant"
		}
		blocks := anthropicBlocks(content)
		if len(blocks) > 0 {
			messages = append(messages, map[string]any{"role": role, "content": blocks})
		}
	}
	return system, messages
}

func anthropicBlocks(content *genai.Content) []map[string]any {
	var blocks []map[string]any
	for _, part := range content.Parts {
		switch {
		case part == nil:
		case part.Text != "":
			blocks = append(blocks, map[string]any{"type": "text", "text": part.Text})
		case part.FunctionCall != nil:
			blocks = append(blocks, map[string]any{
				"type":  "tool_use",
				"id":    fallbackID(part.FunctionCall.ID, part.FunctionCall.Name),
				"name":  part.FunctionCall.Name,
				"input": part.FunctionCall.Args,
			})
		case part.FunctionResponse != nil:
			blocks = append(blocks, map[string]any{
				"type":        "tool_result",
				"tool_use_id": fallbackID(part.FunctionResponse.ID, part.FunctionResponse.Name),
				"content":     mustJSON(part.FunctionResponse.Response),
			})
		}
	}
	return blocks
}

func anthropicTools(req *model.LLMRequest) []map[string]any {
	var tools []map[string]any
	for _, decl := range functionDeclarations(req) {
		tools = append(tools, map[string]any{
			"name":         decl.Name,
			"description":  decl.Description,
			"input_schema": schemaObject(decl.ParametersJsonSchema, decl.Parameters),
		})
	}
	return tools
}

func anthropicToLLM(out anthropicResponse) *model.LLMResponse {
	var parts []*genai.Part
	for _, block := range out.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, genai.NewPartFromText(block.Text))
			}
		case "tool_use":
			parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{
				ID:   block.ID,
				Name: block.Name,
				Args: block.Input,
			}})
		}
	}
	return &model.LLMResponse{Content: genai.NewContentFromParts(parts, genai.RoleModel)}
}

func functionDeclarations(req *model.LLMRequest) []*genai.FunctionDeclaration {
	if req == nil || req.Config == nil {
		return nil
	}
	var decls []*genai.FunctionDeclaration
	for _, tool := range req.Config.Tools {
		if tool != nil {
			decls = append(decls, tool.FunctionDeclarations...)
		}
	}
	return decls
}

func schemaObject(jsonSchema any, schema *genai.Schema) any {
	if jsonSchema != nil {
		return jsonSchema
	}
	if schema != nil {
		return schema
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var parts []string
	for _, part := range content.Parts {
		if part != nil && part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func systemInstruction(req *model.LLMRequest) *genai.Content {
	if req == nil || req.Config == nil {
		return nil
	}
	return req.Config.SystemInstruction
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func fallbackID(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

func readSSE(r io.Reader, handle func(data string) bool) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if !handle(data) {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read model stream: %w", err)
	}
	return nil
}

func textResponse(text string, partial bool) *model.LLMResponse {
	return &model.LLMResponse{
		Content:      genai.NewContentFromText(text, genai.RoleModel),
		Partial:      partial,
		TurnComplete: !partial,
	}
}

func toolCallResponse(calls []*genai.FunctionCall) *model.LLMResponse {
	parts := make([]*genai.Part, 0, len(calls))
	for _, call := range calls {
		parts = append(parts, &genai.Part{FunctionCall: call})
	}
	return &model.LLMResponse{
		Content:      genai.NewContentFromParts(parts, genai.RoleModel),
		TurnComplete: true,
	}
}

func orderedToolCalls(calls map[int]*streamingToolCall) []*genai.FunctionCall {
	result := make([]*genai.FunctionCall, 0, len(calls))
	for i := 0; i < len(calls); i++ {
		call, ok := calls[i]
		if !ok {
			continue
		}
		args := map[string]any{}
		if raw := call.arguments.String(); raw != "" {
			_ = json.Unmarshal([]byte(raw), &args)
		}
		result = append(result, &genai.FunctionCall{
			ID:   call.id,
			Name: call.name,
			Args: args,
		})
	}
	return result
}

type streamingToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIMessage struct {
	Content   string           `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

type anthropicResponse struct {
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Text  string         `json:"text"`
	Input map[string]any `json:"input"`
}

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}
