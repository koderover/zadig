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
	return &commonmodels.CustomNavigation{Items: getDefaultSystemNavigation()}, nil
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

func getDefaultSystemNavigation() []*commonmodels.NavigationItem {
	return []*commonmodels.NavigationItem{
		{
			Name:     "产品交付",
			EnName:   "Product Delivery",
			Key:      "productDelivery",
			Type:     "folder",
			IconType: "class",
			Icon:     "iconfont iconcate",
			Children: []*commonmodels.NavigationItem{
				{
					Name:     "运行状态",
					EnName:   "Status",
					Key:      "status",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconyunhangzhuangtai",
					URL:      "status",
				},
				{
					Name:     "项目",
					EnName:   "Projects",
					Key:      "projects",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconxiangmuloading",
					URL:      "projects",
				},
			},
		},
		{
			Name:     "发布管理",
			EnName:   "Release Management",
			Key:      "releaseManagement",
			Type:     "folder",
			IconType: "class",
			Icon:     "iconfont iconcate",
			Children: []*commonmodels.NavigationItem{
				{
					Name:     "工作流",
					EnName:   "Workflows",
					Key:      "releaseWorkflow",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont icongongzuoliucheng",
					URL:      "releaseWorkflow",
				},
				{
					Name:     "发布计划",
					EnName:   "Plan",
					Key:      "releasePlan",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconplan",
					URL:      "releasePlan",
				},
				{
					Name:     "客户交付",
					EnName:   "Customer Delivery",
					Key:      "customerDelivery",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconjiaofu",
					URL:      "plutus",
				},
			},
		},
		{
			Name:     "资产管理",
			EnName:   "Assets",
			Key:      "assetManagement",
			Type:     "folder",
			IconType: "class",
			Icon:     "iconfont iconcate",
			Children: []*commonmodels.NavigationItem{
				{
					Name:     "业务目录",
					EnName:   "Service Catalog",
					Key:      "bizCatalog",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconDirectorytree",
					URL:      "directory",
				},
				{
					Name:     "模板库",
					EnName:   "Templates",
					Key:      "templateLibrary",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconvery-template",
					URL:      "template",
				},
				{
					Name:     "质量中心",
					EnName:   "Quality",
					Key:      "qualityCenter",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconvery-testing",
					URL:      "tests",
				},
				{
					Name:     "制品管理",
					EnName:   "Artifacts",
					Key:      "artifactManagement",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconvery-deli",
					URL:      "delivery",
				},
				{
					Name:     "资源配置",
					EnName:   "Resourcing",
					Key:      "resourceConfiguration",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconresource-mgr",
					URL:      "resource",
				},
			},
		},
		{
			Name:     "数据视图",
			EnName:   "Statistics",
			Key:      "dataView",
			Type:     "folder",
			IconType: "class",
			Icon:     "iconfont iconcate",
			Children: []*commonmodels.NavigationItem{
				{
					Name:     "数据概览",
					EnName:   "Overview",
					Key:      "dataOverview",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconvery-dataov",
					URL:      "statistics",
				},
				{
					Name:     "效能洞察",
					EnName:   "Insights",
					Key:      "dataInsight",
					Type:     "page",
					PageType: "system",
					IconType: "class",
					Icon:     "iconfont iconvery-datain",
					URL:      "insight",
				},
			},
		},
	}
}
