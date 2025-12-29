/*
Copyright 2024 The KodeRover Authors.

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
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type KeyVaultGroup struct {
	Name string                       `json:"name"`
	KVs  []*commonmodels.KeyVaultItem `json:"kvs"`
}

type KeyVaultListResponse struct {
	Groups []*KeyVaultGroup `json:"groups"`
}

// ListAvailableKeyVaultItemsForProject returns merged KV items for a project
// It merges project-specific items with system-wide items, where project items take priority
// If getSensitiveValue is true, sensitive values are returned; otherwise they are removed
func ListAvailableKeyVaultItemsForProject(projectName string, getSensitiveValue bool) (*KeyVaultListResponse, error) {
	// 1. List all KV pairs for the project
	projectItems, err := commonrepo.NewKeyVaultItemColl().List(&commonrepo.KeyVaultItemListOption{
		ProjectName: projectName,
	})
	if err != nil {
		log.Errorf("KeyVaultItem.List for project %s error: %s", projectName, err)
		return nil, e.ErrListKeyVaultItem.AddErr(err)
	}

	// 2. List all system-wide KV pairs
	systemItems, err := commonrepo.NewKeyVaultItemColl().List(&commonrepo.KeyVaultItemListOption{
		IsSystemWide: true,
	})
	if err != nil {
		log.Errorf("KeyVaultItem.List for system-wide error: %s", err)
		return nil, e.ErrListKeyVaultItem.AddErr(err)
	}

	// 3. Merge with project items taking priority
	mergedMap := make(map[string]*commonmodels.KeyVaultItem)
	groupOrder := make([]string, 0)
	groupSet := make(map[string]bool)

	// Add system items first (lower priority)
	for _, item := range systemItems {
		key := item.Group + ":" + item.Key
		mergedMap[key] = item
		if !groupSet[item.Group] {
			groupOrder = append(groupOrder, item.Group)
			groupSet[item.Group] = true
		}
	}

	// Override with project items (higher priority)
	for _, item := range projectItems {
		key := item.Group + ":" + item.Key
		mergedMap[key] = item
		if !groupSet[item.Group] {
			groupOrder = append(groupOrder, item.Group)
			groupSet[item.Group] = true
		}
	}

	// Handle sensitive values and group by group name
	groupMap := make(map[string][]*commonmodels.KeyVaultItem)
	for _, item := range mergedMap {
		if !getSensitiveValue && item.IsSensitive {
			item.Value = ""
		}
		groupMap[item.Group] = append(groupMap[item.Group], item)
	}

	// Build response maintaining group order
	groups := make([]*KeyVaultGroup, 0, len(groupOrder))
	for _, groupName := range groupOrder {
		if kvs, exists := groupMap[groupName]; exists {
			groups = append(groups, &KeyVaultGroup{
				Name: groupName,
				KVs:  kvs,
			})
		}
	}

	return &KeyVaultListResponse{Groups: groups}, nil
}
