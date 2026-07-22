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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

type Provider string

const (
	ProviderOpenAI               Provider = "openai"
	ProviderDeepSeek             Provider = "deepseek"
	ProviderDeepSeekSiliconCloud Provider = "deepseek_siliconcloud"
	ProviderAzure                Provider = "azure_openai"
	ProviderAzureAD              Provider = "azure_ad_openai"
	ProviderAliyunBailian        Provider = "bailian"
	ProviderVolcengineArk        Provider = "ark"
	ProviderHuaweiMaas           Provider = "maas"
	ProviderOther                Provider = "other"
)

type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
)

type ILLM interface {
	Configure(config LLMConfig) error
	GetCompletion(ctx context.Context, prompt string, options ...ParamOption) (string, error)
	Parse(ctx context.Context, prompt string, cache cache.ICache, options ...ParamOption) (string, error)
	GetName() string
	GetModel() string
}

func NewClientByProtocol(protocol Protocol) (ILLM, error) {
	switch protocol {
	case "", ProtocolOpenAI:
		return &OpenAIClient{}, nil
	case ProtocolAnthropic:
		return &AnthropicClient{}, nil
	default:
		return nil, fmt.Errorf("protocol %s is not supported", protocol)
	}
}

type LLMConfig struct {
	Name         string
	Protocol     Protocol
	ProviderName Provider
	Model        string
	Token        string
	BaseURL      string
	Proxy        string
}

func (p *LLMConfig) GetIntegrationName() string {
	return p.Name
}

func (p *LLMConfig) GetProviderName() Provider {
	return p.ProviderName
}

func (p *LLMConfig) GetBaseURL() string {
	return p.BaseURL
}

func (p *LLMConfig) GetToken() string {
	return p.Token
}

func (p *LLMConfig) GetModel() string {
	return p.Model
}

func (p *LLMConfig) GetProxy() string {
	return p.Proxy
}

func GetCacheKeyWithModel(provider, model, sEnc string) string {
	data := strings.Join([]string{provider, model, sEnc}, "\x00")

	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}

func cacheNamespace(protocol Protocol, integrationName string, provider Provider) string {
	if integrationName == "" {
		integrationName = string(provider)
	}
	return string(protocol) + ":" + integrationName
}
