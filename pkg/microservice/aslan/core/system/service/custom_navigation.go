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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func GetSystemNavigation(log *zap.SugaredLogger) (*commonmodels.CustomNavigation, error) {
	repo := commonrepo.NewCustomNavigationColl()
	res, err := repo.Get()
	if err == nil && res != nil {
		return res, nil
	}
	return &commonmodels.CustomNavigation{Items: make([]*commonmodels.NavigationItem, 0)}, nil
}

func UpdateSystemNavigation(updateBy string, items []*commonmodels.NavigationItem, log *zap.SugaredLogger) error {
	nav := &commonmodels.CustomNavigation{
		Items:    items,
		UpdateBy: updateBy,
	}
	if err := nav.Validate(); err != nil {
		return err
	}
	return commonrepo.NewCustomNavigationColl().CreateOrUpdate(nav)
}
