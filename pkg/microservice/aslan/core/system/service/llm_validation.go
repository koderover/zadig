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

package service

import (
	"fmt"
	"net/url"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func normalizeLLMIntegration(args *commonmodels.LLMIntegration) {
	if args == nil {
		return
	}
	args.Name = strings.TrimSpace(args.Name)
	args.Model = strings.TrimSpace(args.Model)
	args.BaseURL = strings.TrimSpace(args.BaseURL)
	if args.Protocol == "" {
		args.Protocol = llm.ProtocolOpenAI
	}
	if args.Name == "" {
		args.Name = args.Model
		if args.Name == "" {
			args.Name = string(args.ProviderName)
		}
	}
}

func validateLLMIntegration(args *commonmodels.LLMIntegration) error {
	if args == nil {
		return fmt.Errorf("llm integration is required")
	}
	if args.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !supportedLLMProvider(args.ProviderName) {
		return fmt.Errorf("provider %s is not supported", args.ProviderName)
	}
	if args.Protocol != llm.ProtocolOpenAI && args.Protocol != llm.ProtocolAnthropic {
		return fmt.Errorf("protocol %s is not supported", args.Protocol)
	}
	if args.Protocol == llm.ProtocolAnthropic && args.Model == "" {
		return fmt.Errorf("model is required for anthropic protocol")
	}
	if args.Token == "" {
		return fmt.Errorf("token is required")
	}
	if args.BaseURL != "" {
		parsedURL, err := url.ParseRequestURI(args.BaseURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("base_url is invalid")
		}
	}
	return nil
}

func supportedLLMProvider(provider llm.Provider) bool {
	switch provider {
	case llm.ProviderOpenAI,
		llm.ProviderDeepSeek,
		llm.ProviderDeepSeekSiliconCloud,
		llm.ProviderAzure,
		llm.ProviderAzureAD,
		llm.ProviderAliyunBailian,
		llm.ProviderVolcengineArk,
		llm.ProviderHuaweiMaas,
		llm.ProviderOther:
		return true
	default:
		return false
	}
}

func shouldSetDefault(existingCount int64, requested bool) bool {
	return existingCount == 0 || requested
}

func validateLLMIntegrationDeletion(integration *commonmodels.LLMIntegration, count int64) error {
	if integration.IsDefault && count > 1 {
		return fmt.Errorf("set another model as default before deleting the current default model")
	}
	return nil
}
