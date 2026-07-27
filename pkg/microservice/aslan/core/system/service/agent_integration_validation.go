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

func normalizeAgentIntegration(integration *commonmodels.AgentIntegration) {
	if integration == nil {
		return
	}
	integration.Name = strings.TrimSpace(integration.Name)
	integration.Description = strings.TrimSpace(integration.Description)
	integration.BaseURL = strings.TrimRight(strings.TrimSpace(integration.BaseURL), "/")
	integration.Model = strings.TrimSpace(integration.Model)
	if integration.Model == "" {
		integration.Model = integration.Name
	}
	integration.APIKey = strings.TrimSpace(integration.APIKey)
	integration.AccessKey = strings.TrimSpace(integration.AccessKey)
	integration.SecretKey = strings.TrimSpace(integration.SecretKey)
}

func validateAgentIntegration(integration *commonmodels.AgentIntegration) error {
	if integration == nil {
		return fmt.Errorf("agent integration is required")
	}
	if integration.Name == "" {
		return fmt.Errorf("name is required")
	}
	if integration.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	parsedURL, err := url.ParseRequestURI(integration.BaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("base_url is invalid")
	}
	if integration.Protocol != llm.ProtocolOpenAI && integration.Protocol != llm.ProtocolAnthropic {
		return fmt.Errorf("protocol %s is not supported", integration.Protocol)
	}
	switch integration.AuthType {
	case commonmodels.AgentAuthTypeAPIKey:
		if integration.APIKey == "" {
			return fmt.Errorf("api_key is required for api_key authentication")
		}
	case commonmodels.AgentAuthTypeAKSK:
		if integration.AccessKey == "" || integration.SecretKey == "" {
			return fmt.Errorf("access_key and secret_key are required for ak_sk authentication")
		}
	default:
		return fmt.Errorf("auth_type %s is not supported", integration.AuthType)
	}
	return nil
}
