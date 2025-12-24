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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type KeyVaultGroup = commonservice.KeyVaultGroup
type KeyVaultListResponse = commonservice.KeyVaultListResponse

// ListKeyVaultItems returns items grouped by group with sensitive values masked
func ListKeyVaultItems(projectName string, isSystemVariable bool, log *zap.SugaredLogger) (*KeyVaultListResponse, error) {
	opt := &commonrepo.KeyVaultItemListOption{
		ProjectName:  projectName,
		IsSystemWide: isSystemVariable,
	}

	items, err := commonrepo.NewKeyVaultItemColl().List(opt)
	if err != nil {
		log.Errorf("KeyVaultItem.List error: %s", err)
		return nil, e.ErrListKeyVaultItem.AddErr(err)
	}

	groupMap := make(map[string][]*commonmodels.KeyVaultItem)
	groupOrder := make([]string, 0)

	for _, item := range items {
		if item.IsSensitive {
			item.Value = ""
		}

		if _, exists := groupMap[item.Group]; !exists {
			groupOrder = append(groupOrder, item.Group)
		}
		groupMap[item.Group] = append(groupMap[item.Group], item)
	}

	groups := make([]*KeyVaultGroup, 0, len(groupOrder))
	for _, groupName := range groupOrder {
		groups = append(groups, &KeyVaultGroup{
			Name: groupName,
			KVs:  groupMap[groupName],
		})
	}

	return &KeyVaultListResponse{Groups: groups}, nil
}

// GetKeyVaultItem returns a single item with decrypted value (including sensitive)
func GetKeyVaultItem(id string, log *zap.SugaredLogger) (*commonmodels.KeyVaultItem, error) {
	item, err := commonrepo.NewKeyVaultItemColl().FindByID(id)
	if err != nil {
		log.Errorf("KeyVaultItem.Find %s error: %s", id, err)
		return nil, e.ErrGetKeyVaultItem.AddErr(err)
	}

	return item, nil
}

// CreateKeyVaultItem creates a new keyvault item
func CreateKeyVaultItem(args *commonmodels.KeyVaultItem, log *zap.SugaredLogger) error {
	err := commonrepo.NewKeyVaultItemColl().Create(args)
	if err != nil {
		log.Errorf("KeyVaultItem.Create error: %s", err)
		return e.ErrCreateKeyVaultItem.AddErr(err)
	}
	return nil
}

// UpdateKeyVaultItem updates an existing keyvault item
func UpdateKeyVaultItem(id string, args *commonmodels.KeyVaultItem, log *zap.SugaredLogger) error {
	err := commonrepo.NewKeyVaultItemColl().Update(id, args)
	if err != nil {
		log.Errorf("KeyVaultItem.Update %s error: %s", id, err)
		return e.ErrUpdateKeyVaultItem.AddErr(err)
	}
	return nil
}

// DeleteKeyVaultItem deletes a keyvault item by ID
func DeleteKeyVaultItem(id string, log *zap.SugaredLogger) error {
	err := commonrepo.NewKeyVaultItemColl().Delete(id)
	if err != nil {
		log.Errorf("KeyVaultItem.Delete %s error: %s", id, err)
		return e.ErrDeleteKeyVaultItem.AddErr(err)
	}
	return nil
}
