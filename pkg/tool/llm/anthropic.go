/*
Copyright 2026 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	defaultAnthropicBaseURL   = "https://api.anthropic.com"
	defaultAnthropicMaxTokens = 4096
	anthropicAPIVersion       = "2023-06-01"
)

type AnthropicClient struct {
	name            string
	integrationName string
	model           string
	token           string
	baseURL         string
	httpClient      *http.Client
}

type anthropicMessageRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	Temperature   *float32           `json:"temperature,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessageResponse struct {
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *AnthropicClient) Configure(config LLMConfig) error {
	baseURL := strings.TrimRight(config.GetBaseURL(), "/")
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return fmt.Errorf("invalid anthropic base url %q", baseURL)
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	if config.GetProxy() != "" {
		proxyURL, err := url.Parse(config.GetProxy())
		if err != nil {
			return fmt.Errorf("invalid proxy url %s", config.GetProxy())
		}
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	c.name = string(config.GetProviderName())
	c.integrationName = config.GetIntegrationName()
	c.model = config.GetModel()
	c.token = config.GetToken()
	c.baseURL = baseURL
	c.httpClient = httpClient
	return nil
}

func (c *AnthropicClient) GetCompletion(ctx context.Context, prompt string, options ...ParamOption) (string, error) {
	opts := ParamOptions{}
	for _, opt := range options {
		opt(&opts)
	}
	opts = ValidOptions(opts)

	model := opts.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return "", errors.New("anthropic model is required")
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	request := &anthropicMessageRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		Messages:      []anthropicMessage{{Role: "user", Content: prompt}},
		StopSequences: opts.StopWords,
	}
	if opts.hasTemperature() {
		request.Temperature = &opts.Temperature
	}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal anthropic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL(), bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("create anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create anthropic message failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return "", fmt.Errorf("anthropic request failed with status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("anthropic request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	response := new(anthropicMessageResponse)
	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}
	var result strings.Builder
	for _, content := range response.Content {
		if content.Type == "text" {
			result.WriteString(content.Text)
		}
	}
	if result.Len() == 0 {
		return "", errors.New("anthropic response contains no text content")
	}
	return result.String(), nil
}

func (c *AnthropicClient) Parse(ctx context.Context, prompt string, cache cache.ICache, options ...ParamOption) (string, error) {
	model := c.GetModel()
	opts := ParamOptions{}
	for _, opt := range options {
		opt(&opts)
	}
	if opts.Model != "" {
		model = opts.Model
	}
	cacheKey := GetCacheKeyWithModel(cacheNamespace(ProtocolAnthropic, c.integrationName, Provider(c.GetName())), model, prompt)
	if !cache.IsCacheDisabled() && cache.Exists(cacheKey) {
		response, err := cache.Load(cacheKey)
		if err != nil {
			return "", err
		}
		if response != "" {
			output, err := base64.StdEncoding.DecodeString(response)
			if err != nil {
				return "", fmt.Errorf("decode cached anthropic response: %w", err)
			}
			return string(output), nil
		}
	}

	response, err := c.GetCompletion(ctx, prompt, options...)
	if err != nil {
		return "", err
	}
	if !cache.IsCacheDisabled() {
		if err := cache.Store(cacheKey, base64.StdEncoding.EncodeToString([]byte(response))); err != nil {
			log.Errorf("error storing anthropic response: %v", err)
		}
	}
	return response, nil
}

func (c *AnthropicClient) GetName() string {
	if c.name == "" {
		return string(ProviderOther)
	}
	return c.name
}

func (c *AnthropicClient) GetModel() string {
	return c.model
}

func (c *AnthropicClient) messagesURL() string {
	switch {
	case strings.HasSuffix(c.baseURL, "/v1/messages"):
		return c.baseURL
	case strings.HasSuffix(c.baseURL, "/v1"):
		return c.baseURL + "/messages"
	default:
		return c.baseURL + "/v1/messages"
	}
}
