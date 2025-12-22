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

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetSystemNavigation(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	resp, err := service.GetSystemNavigation(ctx.Logger)
	if err != nil {
		ctx.RespErr = fmt.Errorf("get system navigation failed: err %s", err)
		return
	}
	filteredItems := filterNavigationItems(ctx, resp.Items)
	resp.Items = filteredItems
	ctx.Resp = resp
}

func filterNavigationItems(ctx *internalhandler.Context, items []*commonmodels.NavigationItem) []*commonmodels.NavigationItem {
	if ctx.Resources.IsSystemAdmin {
		return filterNavigationItemsForAdmin(ctx, items)
	}
	return filterNavigationItemsForNonAdmin(ctx, items)
}

// isProfessionalLicenseRequiredKey returns true if the given key requires a professional license
func isProfessionalLicenseRequiredKey(key config.NavigationItemKey) bool {
	switch key {
	case config.NavigationKeyWorkflows,
		config.NavigationKeyReleasePlan,
		config.NavigationKeyBizCatalog,
		config.NavigationKeyDataInsight:
		return true
	default:
		return false
	}
}

func filterNavigationItemsForAdmin(ctx *internalhandler.Context, items []*commonmodels.NavigationItem) []*commonmodels.NavigationItem {
	newItems := make([]*commonmodels.NavigationItem, 0)
	for _, item := range items {
		if item.Type == config.NavigationItemTypeFolder {
			item.Children = filterNavigationItemsForAdmin(ctx, item.Children)
			newItems = append(newItems, item)
		} else {
			if item.PageType == config.NavigationPageTypePlugin {
				if err := util.CheckZadigProfessionalLicense(); err != nil {
					item.Disabled = true
				}
				newItems = append(newItems, item)
			} else {
				switch item.Key {
				case config.NavigationKeyCustomerDelivery:
					if err := util.CheckZadigLicenseFeatureDelivery(); err != nil {
						continue
					}
					newItems = append(newItems, item)
				default:
					if isProfessionalLicenseRequiredKey(item.Key) {
						if err := util.CheckZadigProfessionalLicense(); err != nil {
							item.Disabled = true
						}
					}
					newItems = append(newItems, item)
				}
			}
		}
	}
	return newItems
}

func filterNavigationItemsForNonAdmin(ctx *internalhandler.Context, items []*commonmodels.NavigationItem) []*commonmodels.NavigationItem {
	newItems := make([]*commonmodels.NavigationItem, 0)
	for _, item := range items {
		if item.Type == config.NavigationItemTypeFolder {
			item.Children = filterNavigationItemsForNonAdmin(ctx, item.Children)
			newItems = append(newItems, item)
		} else {
			if item.PageType == config.NavigationPageTypePlugin {
				if err := util.CheckZadigProfessionalLicense(); err != nil {
					item.Disabled = true
				}
				newItems = append(newItems, item)
			} else {
				switch item.Key {
				case config.NavigationKeyCustomerDelivery:
					// non-admin users cannot access customer delivery
					continue
				default:
					if isProfessionalLicenseRequiredKey(item.Key) {
						if err := util.CheckZadigProfessionalLicense(); err != nil {
							item.Disabled = true
						}
					}
					// check authorization
					if !hasNavigationPermission(ctx, item.Key) {
						item.Disabled = true
					}
					newItems = append(newItems, item)
				}
			}
		}
	}
	return newItems
}

// hasNavigationPermission checks if the user has permission to access the given navigation key
func hasNavigationPermission(ctx *internalhandler.Context, key config.NavigationItemKey) bool {
	switch key {
	case config.NavigationKeyReleasePlan:
		return ctx.Resources.SystemActions.ReleasePlan.View
	case config.NavigationKeyBizCatalog:
		return ctx.Resources.SystemActions.BusinessDirectory.View
	case config.NavigationKeyTemplateLibrary:
		return ctx.Resources.SystemActions.Template.View
	case config.NavigationKeyQualityCenter:
		return ctx.Resources.SystemActions.TestCenter.View
	case config.NavigationKeyArtifactManagement:
		return ctx.Resources.SystemActions.DeliveryCenter.ViewArtifact || ctx.Resources.SystemActions.DeliveryCenter.ViewVersion
	case config.NavigationKeyResourceConfiguration:
		return ctx.Resources.SystemActions.ClusterManagement.View ||
			ctx.Resources.SystemActions.VMManagement.View ||
			ctx.Resources.SystemActions.RegistryManagement.View ||
			ctx.Resources.SystemActions.S3StorageManagement.View ||
			ctx.Resources.SystemActions.HelmRepoManagement.View ||
			ctx.Resources.SystemActions.DBInstanceManagement.View
	case config.NavigationKeyDataOverview:
		return ctx.Resources.SystemActions.DataCenter.ViewOverView
	case config.NavigationKeyDataInsight:
		return ctx.Resources.SystemActions.DataCenter.ViewInsight
	default:
		return true
	}
}

func UpdateSystemNavigation(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	// admin only
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	var body struct {
		Items []*commonmodels.NavigationItem `json:"items"`
	}
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateSystemNavigation c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, &body); err != nil {
		log.Errorf("UpdateSystemNavigation json.Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-自定义导航", "", "", string(data), types.RequestBodyTypeJSON, ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	if err := c.ShouldBindJSON(&body); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid navigation args")
		return
	}
	ctx.RespErr = service.UpdateSystemNavigation(ctx.UserName, body.Items, ctx.Logger)
}
