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
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"k8s.io/apimachinery/pkg/util/sets"
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

func UpdateAgentIntegration(ctx context.Context, projectName, id string, integration *commonmodels.AgentIntegration) error {
	if integration == nil {
		return e.ErrUpdateAgentIntegration.AddErr(fmt.Errorf("agent integration is required"))
	}
	normalizeAgentIntegration(integration)
	current, err := findProjectAgentIntegration(ctx, projectName, id)
	if err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	integration.ProjectName = current.ProjectName
	restoreAgentIntegrationCredentials(current, integration)
	if err := validateAgentIntegration(integration); err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	if err := commonrepo.NewAgentIntegrationColl().Update(ctx, id, integration); err != nil {
		return e.ErrUpdateAgentIntegration.AddErr(err)
	}
	return nil
}

func GetAgentIntegration(ctx context.Context, projectName, id, encryptedKey string) (*commonmodels.AgentIntegration, error) {
	integration, err := findProjectAgentIntegration(ctx, projectName, id)
	if err != nil {
		return nil, e.ErrGetAgentIntegration.AddErr(err)
	}
	if err := protectAgentIntegrationCredentials([]*commonmodels.AgentIntegration{integration}, encryptedKey); err != nil {
		return nil, e.ErrGetAgentIntegration.AddErr(err)
	}
	return integration, nil
}

func ListAgentIntegrations(ctx context.Context, projectName, encryptedKey string) ([]*commonmodels.AgentIntegration, error) {
	integrations, err := commonrepo.NewAgentIntegrationColl().ListByProject(ctx, projectName)
	if err != nil {
		return nil, e.ErrListAgentIntegration.AddErr(err)
	}
	if err := protectAgentIntegrationCredentials(integrations, encryptedKey); err != nil {
		return nil, e.ErrListAgentIntegration.AddErr(err)
	}
	return integrations, nil
}

func protectAgentIntegrationCredentials(integrations []*commonmodels.AgentIntegration, encryptedKey string) error {
	if encryptedKey == "" {
		for _, integration := range integrations {
			if integration.APIKey != "" {
				integration.APIKey = setting.MaskValue
			}
			if integration.AccessKey != "" {
				integration.AccessKey = setting.MaskValue
			}
			if integration.SecretKey != "" {
				integration.SecretKey = setting.MaskValue
			}
		}
		return nil
	}

	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("get aes key from encrypted key: %w", err)
	}
	for _, integration := range integrations {
		for _, credential := range []*string{&integration.APIKey, &integration.AccessKey, &integration.SecretKey} {
			if *credential == "" {
				continue
			}
			encrypted, err := crypto.AesEncryptByKey(*credential, aesKey.PlainText)
			if err != nil {
				return fmt.Errorf("encrypt credential: %w", err)
			}
			*credential = encrypted
		}
	}
	return nil
}

type AgentIntegrationBrief struct {
	ID           string `json:"id"`
	ProjectName  string `json:"project_name"`
	ProjectAlias string `json:"project_alias"`
	Name         string `json:"name"`
	Description  string `json:"description"`
}

// ListAllAgentIntegrationBriefs lists agents across all agent projects for the
// workflow AI task selector; it never exposes endpoint or credential fields.
func ListAllAgentIntegrationBriefs(ctx context.Context) ([]*AgentIntegrationBrief, error) {
	integrations, err := commonrepo.NewAgentIntegrationColl().ListAll(ctx)
	if err != nil {
		return nil, e.ErrListAgentIntegration.AddErr(err)
	}

	projectNameSet := sets.NewString()
	for _, integration := range integrations {
		projectNameSet.Insert(integration.ProjectName)
	}
	projectAliasMap := map[string]string{}
	if projectNameSet.Len() > 0 {
		projects, err := templaterepo.NewProductColl().ListProjectBriefs(projectNameSet.List())
		if err != nil {
			return nil, e.ErrListAgentIntegration.AddErr(fmt.Errorf("list project briefs: %w", err))
		}
		for _, project := range projects {
			projectAliasMap[project.Name] = project.Alias
		}
	}

	briefs := make([]*AgentIntegrationBrief, 0, len(integrations))
	for _, integration := range integrations {
		briefs = append(briefs, &AgentIntegrationBrief{
			ID:           integration.ID.Hex(),
			ProjectName:  integration.ProjectName,
			ProjectAlias: projectAliasMap[integration.ProjectName],
			Name:         integration.Name,
			Description:  integration.Description,
		})
	}
	return briefs, nil
}

func DeleteAgentIntegration(ctx context.Context, projectName, id string) error {
	if _, err := findProjectAgentIntegration(ctx, projectName, id); err != nil {
		return e.ErrDeleteAgentIntegration.AddErr(err)
	}
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

func ValidateAgentIntegration(ctx context.Context, projectName, id string, integration *commonmodels.AgentIntegration) error {
	if integration == nil {
		return fmt.Errorf("验证 Agent 集成失败: agent integration is required")
	}
	normalizeAgentIntegration(integration)
	if id != "" {
		current, err := findProjectAgentIntegration(ctx, projectName, id)
		if err != nil {
			return fmt.Errorf("验证 Agent 集成失败: %s", err)
		}
		restoreAgentIntegrationCredentials(current, integration)
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

func findProjectAgentIntegration(ctx context.Context, projectName, id string) (*commonmodels.AgentIntegration, error) {
	integration, err := commonrepo.NewAgentIntegrationColl().FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find agent integration: %w", err)
	}
	if integration.ProjectName != projectName {
		return nil, fmt.Errorf("agent integration %s does not belong to project %s", id, projectName)
	}
	return integration, nil
}

func restoreAgentIntegrationCredentials(current, integration *commonmodels.AgentIntegration) {
	if integration.AuthType != current.AuthType {
		return
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
	if integration.ProjectName == "" {
		return fmt.Errorf("project name is required")
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
