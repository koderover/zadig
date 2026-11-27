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
	"slices"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"

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
	OmitModel    bool
	Token        string
	BaseURL      string
	Proxy        string
	Headers      map[string]string
	ModelParameters
}

type ModelParameters struct {
	ContextWindow   int    `bson:"context_window" json:"context_window"`
	MaxOutputTokens int    `bson:"max_output_tokens" json:"max_output_tokens"`
	ReasoningEffort string `bson:"reasoning_effort" json:"reasoning_effort"`
}

func ReasoningEfforts(protocol Protocol) []string {
	if protocol == "" || protocol == ProtocolOpenAI {
		return []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	}
	return []string{"low", "medium", "high", "xhigh", "max"}
}

func (p ModelParameters) Validate(protocol Protocol) error {
	if p.ContextWindow < 0 || p.MaxOutputTokens < 0 {
		return fmt.Errorf("context_window and max_output_tokens must be non-negative")
	}
	if p.ContextWindow > 0 && (p.MaxOutputTokens == 0 || p.MaxOutputTokens >= p.ContextWindow) {
		return fmt.Errorf("context_window requires a positive max_output_tokens smaller than context_window")
	}
	if p.ReasoningEffort == "" || slices.Contains(ReasoningEfforts(protocol), p.ReasoningEffort) {
		return nil
	}
	return fmt.Errorf("reasoning_effort %q is invalid for %s", p.ReasoningEffort, protocol)
}

func (p ModelParameters) limitOutputTokens(requested int) int {
	if p.MaxOutputTokens > 0 && (requested <= 0 || requested > p.MaxOutputTokens) {
		return p.MaxOutputTokens
	}
	return requested
}

func (p ModelParameters) checkPromptBudget(ctx context.Context, prompt string, outputTokens int) error {
	if p.ContextWindow == 0 {
		return nil
	}
	tokens, err := model.NewSimpleTokenCounter().CountTokens(ctx, model.NewUserMessage(prompt))
	if err != nil {
		return err
	}
	inputBudget := p.ContextWindow - outputTokens
	if tokens > inputBudget {
		return fmt.Errorf("prompt exceeds the configured model input budget of %d tokens", inputBudget)
	}
	return nil
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

func (p *LLMConfig) GetHeaders() map[string]string {
	return p.Headers
}

func GetCacheKeyWithModel(provider, model, sEnc string) string {
	data := strings.Join([]string{provider, model, sEnc}, "\x00")

	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}

func cacheNamespace(protocol Protocol, integrationName string, provider Provider, parameters ModelParameters) string {
	if integrationName == "" {
		integrationName = string(provider)
	}
	namespace := string(protocol) + ":" + integrationName
	if parameters != (ModelParameters{}) {
		return fmt.Sprintf("%s\x00%d\x00%d\x00%s", namespace, parameters.ContextWindow, parameters.MaxOutputTokens, parameters.ReasoningEffort)
	}
	return namespace
}
