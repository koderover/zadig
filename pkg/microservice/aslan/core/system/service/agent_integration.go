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
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func CreateAgentIntegration(ctx context.Context, integration *commonmodels.AgentIntegration) error {
	normalizeAgentIntegration(integration)
	if err := validateAgentIntegration(integration); err != nil {
		return e.ErrCreateAgentIntegration.AddErr(err)
	}
	if err := commonrepo.NewAgentIntegrationColl().Create(ctx, integration); err != nil {
		return e.ErrCreateAgentIntegration.AddErr(err)
	}
	return nil
}

func UpdateAgentIntegration(ctx context.Context, id string, integration *commonmodels.AgentIntegration) error {
	normalizeAgentIntegration(integration)
	if integration == nil {
		return e.ErrUpdateAgentIntegration.AddErr(fmt.Errorf("agent integration is required"))
	}
	if err := restoreAgentIntegrationCredentials(ctx, id, integration); err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	if err := validateAgentIntegration(integration); err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	if err := commonrepo.NewAgentIntegrationColl().Update(ctx, id, integration); err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	return nil
}

func GetAgentIntegration(ctx context.Context, id string) (*commonmodels.AgentIntegration, error) {
	integration, err := commonrepo.NewAgentIntegrationColl().FindByID(ctx, id)
	if err != nil {
		return nil, e.ErrGetAgentIntegration.AddErr(err)
	}
	return newAgentIntegrationResponse(integration), nil
}

func ListAgentIntegrations(ctx context.Context) ([]*commonmodels.AgentIntegration, error) {
	integrations, err := commonrepo.NewAgentIntegrationColl().FindAll(ctx)
	if err != nil {
		return nil, e.ErrListAgentIntegration.AddErr(err)
	}
	for i, integration := range integrations {
		integrations[i] = newAgentIntegrationResponse(integration)
	}
	return integrations, nil
}

func newAgentIntegrationResponse(integration *commonmodels.AgentIntegration) *commonmodels.AgentIntegration {
	response := *integration
	if response.APIKey != "" {
		response.APIKey = setting.MaskValue
	}
	if response.AccessKey != "" {
		response.AccessKey = setting.MaskValue
	}
	if response.SecretKey != "" {
		response.SecretKey = setting.MaskValue
	}
	return &response
}

func DeleteAgentIntegration(ctx context.Context, id string) error {
	workflowNames, err := commonrepo.NewWorkflowV4Coll().ListNamesByAITarget(ctx, config.AITargetTypeAgent, id)
	if err != nil {
		return e.ErrDeleteAgentIntegration.AddErr(fmt.Errorf("find workflows referencing the agent: %w", err))
	}
	if len(workflowNames) > 0 {
		return e.ErrDeleteAgentIntegration.AddErr(fmt.Errorf("agent is still used by workflow: %s", strings.Join(workflowNames, ", ")))
	}
	if err := commonrepo.NewAgentIntegrationColl().Delete(ctx, id); err != nil {
		return e.ErrDeleteAgentIntegration.AddErr(err)
	}
	return nil
}

func ValidateAgentIntegration(ctx context.Context, id string, integration *commonmodels.AgentIntegration) error {
	normalizeAgentIntegration(integration)
	if integration == nil {
		return fmt.Errorf("验证 Agent 集成失败: agent integration is required")
	}
	if id != "" {
		if err := restoreAgentIntegrationCredentials(ctx, id, integration); err != nil {
			return fmt.Errorf("验证 Agent 集成失败: %s", err)
		}
	}
	if err := validateAgentIntegration(integration); err != nil {
		return fmt.Errorf("验证 Agent 集成失败: %s", err)
	}
	client, err := llmservice.NewAgentClient(integration)
	if err != nil {
		return fmt.Errorf("验证 Agent 集成失败: %s", err)
	}
	if _, err := client.GetCompletion(ctx, "Hello"); err != nil {
		return fmt.Errorf("验证 Agent 集成失败: %s", err)
	}
	return nil
}

func restoreAgentIntegrationCredentials(ctx context.Context, id string, integration *commonmodels.AgentIntegration) error {
	current, err := commonrepo.NewAgentIntegrationColl().FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find agent integration: %w", err)
	}
	if integration.AuthType != current.AuthType {
		return nil
	}

	switch integration.AuthType {
	case commonmodels.AgentAuthTypeAPIKey:
		if integration.APIKey == "" || integration.APIKey == setting.MaskValue {
			integration.APIKey = current.APIKey
		}
	case commonmodels.AgentAuthTypeAKSK:
		if integration.AccessKey == "" || integration.AccessKey == setting.MaskValue {
			integration.AccessKey = current.AccessKey
		}
		if integration.SecretKey == "" || integration.SecretKey == setting.MaskValue {
			integration.SecretKey = current.SecretKey
		}
	}
	return nil
}

func normalizeAgentIntegration(integration *commonmodels.AgentIntegration) {
	if integration == nil {
		return
	}
	integration.Name = strings.TrimSpace(integration.Name)
	integration.Description = strings.TrimSpace(integration.Description)
	integration.BaseURL = strings.TrimRight(strings.TrimSpace(integration.BaseURL), "/")
	integration.Model = strings.TrimSpace(integration.Model)
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
	if integration.Protocol == llm.ProtocolAnthropic && integration.Model == "" {
		return fmt.Errorf("model is required for anthropic protocol")
	}
	switch integration.AuthType {
	case commonmodels.AgentAuthTypeAPIKey:
		if integration.APIKey == "" || integration.APIKey == setting.MaskValue {
			return fmt.Errorf("api_key is required for api_key authentication")
		}
	case commonmodels.AgentAuthTypeAKSK:
		if integration.AccessKey == "" || integration.AccessKey == setting.MaskValue || integration.SecretKey == "" || integration.SecretKey == setting.MaskValue {
			return fmt.Errorf("access_key and secret_key are required for ak_sk authentication")
		}
	default:
		return fmt.Errorf("auth_type %s is not supported", integration.AuthType)
	}
	return nil
}
