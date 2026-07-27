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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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
	response.APIKey = ""
	response.AccessKey = ""
	response.SecretKey = ""
	return &response
}

func DeleteAgentIntegration(ctx context.Context, id string) error {
	if err := commonrepo.NewAgentIntegrationColl().Delete(ctx, id); err != nil {
		return e.ErrDeleteAgentIntegration.AddErr(err)
	}
	return nil
}

func ValidateAgentIntegration(ctx context.Context, integration *commonmodels.AgentIntegration) error {
	normalizeAgentIntegration(integration)
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
