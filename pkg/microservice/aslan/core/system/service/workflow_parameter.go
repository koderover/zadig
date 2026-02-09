/*
Copyright 2025 The KodeRover Authors.

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

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
)

// WorkflowParameterItem represents a single workflow parameter for frontend rendering.
// Key is the template variable path (e.g. project.id), Description is the display label.
type WorkflowParameterItem struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type Scope string

const (
	ScopeSystem Scope = "system"
	ScopeProject Scope = "project"
)

// ListAvailableWorkflowParameter returns workflow parameters that can be used in templates.
// It includes keyvault parameters (parameter.group.key) when projectName is set.
func ListAvailableWorkflowParameter(scope, projectName string) (interface{}, error) {
	list := []WorkflowParameterItem{}

	var keyvaultResp *commonservice.KeyVaultListResponse
	var err error

	switch scope {
	case string(ScopeSystem):
		keyvaultResp, err = commonservice.ListAvailableKeyVaultItemsForSystem(false)
		if err != nil {
			return nil, fmt.Errorf("list keyvault items for system: %w", err)
		}
	case string(ScopeProject):
		keyvaultResp, err = commonservice.ListAvailableKeyVaultItemsForProject(projectName, false)
		if err != nil {
			return nil, fmt.Errorf("list keyvault items for project %s: %w", projectName, err)
		}
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}

	for _, group := range keyvaultResp.Groups {
		for _, item := range group.KVs {
			paramKey := fmt.Sprintf("parameter.%s.%s", item.Group, item.Key)
			desc := item.Description
			if desc == "" {
				desc = item.Key
			}
			list = append(list, WorkflowParameterItem{Key: paramKey, Description: desc})
		}
	}

	return list, nil
}