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

	"github.com/koderover/zadig/pkg/tool/cache"
	"github.com/koderover/zadig/pkg/tool/log"

	"github.com/sashabaranov/go-openai"
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

// @todo add ability to set temperature, top_p, frequency_penalty, presence_penalty, stop
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
			model = "gpt-3.5-turbo"
		} else {
			model = c.model
		}
	}

	// Create a completion request
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stop:        opts.StopWords,
	})
	if err != nil {
		return "", err
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
