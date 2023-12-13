/*
Copyright 2023 The K8sGPT Authors.
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

// Some parts of this file have been modified to make it functional in Zadig

package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkoukk/tiktoken-go"
	"github.com/sashabaranov/go-openai"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	DefaultOpenAIModel           = openai.GPT3Dot5Turbo
	DefaultOpenAIModelTokenLimit = "4096"
)

type OpenAIClient struct {
	name    string
	model   string
	client  *openai.Client
	apiType string
}

func (c *OpenAIClient) Configure(config LLMConfig) error {
	token := config.GetToken()
	var defaultConfig openai.ClientConfig
	if config.GetAPIType() == "AZURE" || config.GetAPIType() == "AZURE_AD" {
		c.apiType = "AZURE"
		baseURL := config.GetBaseURL()
		defaultConfig = openai.DefaultAzureConfig(token, baseURL)

		if config.GetAPIType() == "AZURE_AD" {
			c.apiType = "AZURE_AD"
			defaultConfig.APIType = openai.APITypeAzureAD
		}
	} else {
		// config.GetAPIType() == "OPEN_AI"
		c.apiType = "OPEN_AI"
		defaultConfig = openai.DefaultConfig(token)
	}

	if config.GetProxy() != "" {
		proxyUrl, err := url.Parse(config.GetProxy())
		if err != nil {
			return fmt.Errorf("invalid proxy url %s", config.GetProxy())
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		defaultConfig.HTTPClient = &http.Client{
			Transport: transport,
		}
	}

	client := openai.NewClientWithConfig(defaultConfig)
	if client == nil {
		return errors.New("error creating OpenAI client")
	}

	c.client = client
	c.name = config.GetName()
	c.model = config.GetModel()
	return nil
}

// @todo add ability to supply multiple messages
func (c *OpenAIClient) GetCompletion(ctx context.Context, prompt string, options ...ParamOption) (string, error) {
	opts := ParamOptions{}
	for _, opt := range options {
		opt(&opts)
	}
	opts = ValidOptions(opts)

	model := opts.Model
	if model == "" {
		if c.model == "" {
			model = DefaultOpenAIModel
		} else {
			model = c.model
		}
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	var resp openai.ChatCompletionResponse
	var err error
	if opts.MaxTokens == 0 {
		resp, err = c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Temperature: opts.Temperature,
			Stop:        opts.StopWords,
			LogitBias:   opts.LogitBias,
		})
	} else {
		resp, err = c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			MaxTokens:   opts.MaxTokens,
			Temperature: opts.Temperature,
			Stop:        opts.StopWords,
			LogitBias:   opts.LogitBias,
		})
	}

	if err != nil {
		return "", fmt.Errorf("create chat completion failed: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func (a *OpenAIClient) Parse(ctx context.Context, prompt string, cache cache.ICache, options ...ParamOption) (string, error) {
	// Check for cached data
	cacheKey := GetCacheKey(a.GetName(), prompt)

	if !cache.IsCacheDisabled() && cache.Exists(cacheKey) {
		response, err := cache.Load(cacheKey)
		if err != nil {
			return "", err
		}

		if response != "" {
			output, err := base64.StdEncoding.DecodeString(response)
			if err != nil {
				log.Errorf("error decoding cached data: %v", err)
				return "", nil
			}
			return string(output), nil
		}
	}

	response, err := a.GetCompletion(ctx, prompt, options...)
	if err != nil {
		return "", err
	}

	err = cache.Store(cacheKey, base64.StdEncoding.EncodeToString([]byte(response)))

	if err != nil {
		log.Errorf("error storing value to cache: %v", err)
		return "", nil
	}

	return response, nil
}

func (a *OpenAIClient) GetName() string {
	if a.name == "" {
		if a.apiType == "AZURE" || a.apiType == "AZURE_AD" {
			return "azureopenai"
		}
		return "openai"
	}
	return a.name
}

func NumTokensFromMessages(messages []openai.ChatCompletionMessage, model string) (num_tokens int, err error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		err = fmt.Errorf("EncodingForModel error: %w", err)
		return
	}

	var tokens_per_message int
	var tokens_per_name int
	if model == "gpt-3.5-turbo-0301" || model == "gpt-3.5-turbo" {
		tokens_per_message = 4
		tokens_per_name = -1
	} else if model == "gpt-4-0314" || model == "gpt-4" {
		tokens_per_message = 3
		tokens_per_name = 1
	} else {
		tokens_per_message = 3
		tokens_per_name = 1
		log.Warnf("Warning: model not found. Using cl100k_base encoding.")
	}

	for _, message := range messages {
		num_tokens += tokens_per_message
		num_tokens += len(tkm.Encode(message.Content, nil, nil))
		num_tokens += len(tkm.Encode(message.Role, nil, nil))
		num_tokens += len(tkm.Encode(message.Name, nil, nil))
		if message.Name != "" {
			num_tokens += tokens_per_name
		}
	}
	num_tokens += 3
	return num_tokens, nil
}

func NumTokensFromPrompt(prompt string, model string) (num_tokens int, err error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	if model == "" {
		model = DefaultOpenAIModel
	}

	return NumTokensFromMessages(messages, model)
}
