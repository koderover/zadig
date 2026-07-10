package llmbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"hello-agent/internal/llmconfig"
)

func TestOpenAIBridgeSendsToolsAndParsesToolCall(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "lookup_runbook",
							"arguments": "{\"service\":\"payment-api\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}]
		}`))
	}))
	defer server.Close()

	llm := New(llmconfig.Config{
		Provider: ProviderOpenAI,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "ops-model",
		Timeout:  time.Second,
	}, server.Client())

	resp, err := firstResponse(llm.GenerateContent(context.Background(), sampleRequest(), false))
	if err != nil {
		t.Fatalf("GenerateContent returned error: %v", err)
	}

	if captured["model"] != "ops-model" {
		t.Fatalf("model = %v", captured["model"])
	}
	tools, ok := captured["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", captured["tools"])
	}
	if resp.Content.Parts[0].FunctionCall.Name != "lookup_runbook" {
		t.Fatalf("FunctionCall.Name = %q", resp.Content.Parts[0].FunctionCall.Name)
	}
	if resp.Content.Parts[0].FunctionCall.Args["service"] != "payment-api" {
		t.Fatalf("FunctionCall.Args = %#v", resp.Content.Parts[0].FunctionCall.Args)
	}
}

func TestAnthropicBridgeSendsToolsAndParsesToolUse(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("path = %q, want /messages", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Fatalf("x-api-key = %q", got)
		}
		if values, ok := r.Header["Anthropic-Version"]; ok {
			t.Fatalf("anthropic-version header = %#v, want absent", values)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"id": "msg-1",
			"type": "message",
			"role": "assistant",
			"content": [{
				"type": "tool_use",
				"id": "toolu_1",
				"name": "lookup_runbook",
				"input": {"service":"gateway"}
			}],
			"stop_reason": "tool_use"
		}`))
	}))
	defer server.Close()

	llm := New(llmconfig.Config{
		Provider: ProviderAnthropic,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "claude-compatible",
		Timeout:  time.Second,
	}, server.Client())

	resp, err := firstResponse(llm.GenerateContent(context.Background(), sampleRequest(), false))
	if err != nil {
		t.Fatalf("GenerateContent returned error: %v", err)
	}

	if captured["model"] != "claude-compatible" {
		t.Fatalf("model = %v", captured["model"])
	}
	tools, ok := captured["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", captured["tools"])
	}
	if resp.Content.Parts[0].FunctionCall.ID != "toolu_1" {
		t.Fatalf("FunctionCall.ID = %q", resp.Content.Parts[0].FunctionCall.ID)
	}
	if resp.Content.Parts[0].FunctionCall.Args["service"] != "gateway" {
		t.Fatalf("FunctionCall.Args = %#v", resp.Content.Parts[0].FunctionCall.Args)
	}
}

func TestBridgeReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad model", http.StatusBadGateway)
	}))
	defer server.Close()

	llm := New(llmconfig.Config{
		Provider: ProviderOpenAI,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "ops-model",
		Timeout:  time.Second,
	}, server.Client())

	_, err := firstResponse(llm.GenerateContent(context.Background(), sampleRequest(), false))
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v, want HTTP status", err)
	}
}

func TestOpenAIBridgeStreamsTextDeltas(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("content-type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"第一步\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"：查指标\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	llm := New(llmconfig.Config{
		Provider: ProviderOpenAI,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "ops-model",
		Timeout:  time.Second,
	}, server.Client())

	responses, err := collectResponses(llm.GenerateContent(context.Background(), sampleRequest(), true))
	if err != nil {
		t.Fatalf("GenerateContent returned error: %v", err)
	}

	if captured["stream"] != true {
		t.Fatalf("stream = %v, want true", captured["stream"])
	}
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	if !responses[0].Partial || responses[0].Content.Parts[0].Text != "第一步" {
		t.Fatalf("first response = %#v", responses[0])
	}
	if !responses[1].Partial || responses[1].Content.Parts[0].Text != "：查指标" {
		t.Fatalf("second response = %#v", responses[1])
	}
	if responses[2].Partial || responses[2].Content.Parts[0].Text != "第一步：查指标" {
		t.Fatalf("final response = %#v", responses[2])
	}
}

func TestAnthropicBridgeStreamsTextDeltas(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("content-type", "text/event-stream")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"先看\"}}\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" p95\"}}\n\n")
		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer server.Close()

	llm := New(llmconfig.Config{
		Provider: ProviderAnthropic,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "claude-compatible",
		Timeout:  time.Second,
	}, server.Client())

	responses, err := collectResponses(llm.GenerateContent(context.Background(), sampleRequest(), true))
	if err != nil {
		t.Fatalf("GenerateContent returned error: %v", err)
	}

	if captured["stream"] != true {
		t.Fatalf("stream = %v, want true", captured["stream"])
	}
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	if !responses[0].Partial || responses[0].Content.Parts[0].Text != "先看" {
		t.Fatalf("first response = %#v", responses[0])
	}
	if !responses[1].Partial || responses[1].Content.Parts[0].Text != " p95" {
		t.Fatalf("second response = %#v", responses[1])
	}
	if responses[2].Partial || responses[2].Content.Parts[0].Text != "先看 p95" {
		t.Fatalf("final response = %#v", responses[2])
	}
}

func firstResponse(seq func(func(*model.LLMResponse, error) bool)) (*model.LLMResponse, error) {
	var resp *model.LLMResponse
	var err error
	for got, gotErr := range seq {
		resp, err = got, gotErr
		break
	}
	return resp, err
}

func collectResponses(seq func(func(*model.LLMResponse, error) bool)) ([]*model.LLMResponse, error) {
	var responses []*model.LLMResponse
	for got, err := range seq {
		if err != nil {
			return responses, err
		}
		responses = append(responses, got)
	}
	return responses, nil
}

func sampleRequest() *model.LLMRequest {
	return &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("payment-api latency is high", genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			Tools: []*genai.Tool{{
				FunctionDeclarations: []*genai.FunctionDeclaration{{
					Name:        "lookup_runbook",
					Description: "Look up DevOps runbook for a service.",
					ParametersJsonSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"service": map[string]any{"type": "string"},
						},
						"required": []any{"service"},
					},
				}},
			}},
		},
	}
}
