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

package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/koderover/zadig/pkg/tool/cache"
)

var (
	clients = map[string]IAI{
		"openai":      &OpenAIClient{},
		"azureopenai": &OpenAIClient{},
	}
)

type IAI interface {
	Configure(config AIConfig) error
	GetCompletion(ctx context.Context, prompt string, options ...ParamOption) (string, error)
	Parse(ctx context.Context, prompt string, cache cache.ICache, options ...ParamOption) (string, error)
	GetName() string
}

func NewClient(provider string) IAI {
	if c, ok := clients[provider]; !ok {
		return &OpenAIClient{}
	} else {
		return c
	}
}

type AIConfig struct {
	Name    string
	Model   string
	Token   string
	BaseURL string
	Proxy   string
	APIType string
	Engine  string
}

func (p *AIConfig) GetName() string {
	return p.Name
}

func (p *AIConfig) GetBaseURL() string {
	return p.BaseURL
}

func (p *AIConfig) GetToken() string {
	return p.Token
}

func (p *AIConfig) GetModel() string {
	return p.Model
}

func (p *AIConfig) GetProxy() string {
	return p.Proxy
}

func (p *AIConfig) GetAPIType() string {
	return p.APIType
}

func (p *AIConfig) GetEngine() string {
	return p.Engine
}

func GetCacheKey(provider string, sEnc string) string {
	data := fmt.Sprintf("%s-%s", provider, sEnc)

	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}
