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

package handler

import (
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// canEditSystemIntegration reports whether the user may see system integration secrets.
// Users with view permission only (e.g. global read-only) get masked secrets.
func canEditSystemIntegration(ctx *internalhandler.Context) bool {
	if ctx == nil || ctx.Resources == nil {
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	if ctx.Resources.SystemActions == nil || ctx.Resources.SystemActions.SystemIntegration == nil {
		return false
	}
	actions := ctx.Resources.SystemActions.SystemIntegration
	return actions.Create || actions.Edit
}

func maskSecret(value *string) {
	if value != nil && *value != "" {
		*value = setting.MaskValue
	}
}

func maskExternalSystemSecrets(systems ...*service.ExternalSystemDetail) {
	for _, system := range systems {
		if system == nil {
			continue
		}
		maskSecret(&system.APIToken)
		for _, header := range system.Headers {
			if header != nil && header.Value != nil && header.Value != "" {
				header.Value = setting.MaskValue
			}
		}
	}
}

func maskCICDToolSecrets(tools []*commonmodels.JenkinsIntegration) {
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		maskSecret(&tool.Password)
		maskSecret(&tool.AppSecret)
	}
}

func maskCloudServiceSecrets(clouds ...*commonmodels.CloudService) {
	for _, cloud := range clouds {
		if cloud == nil {
			continue
		}
		maskSecret(&cloud.AccessKeySecret)
	}
}

func maskProjectManagementSecrets(pms []*commonmodels.ProjectManagement) error {
	for _, pm := range pms {
		if pm == nil {
			continue
		}
		maskSecret(&pm.JiraToken)
		maskSecret(&pm.JiraPersonalAccessToken)
		maskSecret(&pm.MeegoPluginSecret)
		if pm.Spec == nil {
			continue
		}

		switch pm.Type {
		case setting.ProjectManagementTypeJira:
			spec := &commonmodels.ProjectManagementJiraSpec{}
			if err := commonmodels.IToi(pm.Spec, spec); err != nil {
				return err
			}
			maskSecret(&spec.JiraToken)
			maskSecret(&spec.JiraPersonalAccessToken)
			pm.Spec = spec
		case setting.ProjectManagementTypeMeego:
			spec := &commonmodels.ProjectManagementMeegoSpec{}
			if err := commonmodels.IToi(pm.Spec, spec); err != nil {
				return err
			}
			maskSecret(&spec.MeegoPluginSecret)
			pm.Spec = spec
		case setting.ProjectManagementTypePingCode:
			spec := &commonmodels.ProjectManagementPingCodeSpec{}
			if err := commonmodels.IToi(pm.Spec, spec); err != nil {
				return err
			}
			maskSecret(&spec.PingCodeClientSecret)
			pm.Spec = spec
		case setting.ProjectManagementTypeTapd:
			spec := &commonmodels.ProjectManagementTapdSpec{}
			if err := commonmodels.IToi(pm.Spec, spec); err != nil {
				return err
			}
			maskSecret(&spec.TapdClientSecret)
			pm.Spec = spec
		}
	}
	return nil
}

func maskLLMIntegrationTokens(integrations ...*commonmodels.LLMIntegration) {
	for _, integration := range integrations {
		if integration == nil {
			continue
		}
		maskSecret(&integration.Token)
	}
}
