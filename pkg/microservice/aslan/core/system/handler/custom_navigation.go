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
		newItem := make([]*commonmodels.NavigationItem, 0)
		for _, item := range items {
			if item.Type == config.NavigationItemTypeFolder {
				item.Children = filterNavigationItems(ctx, item.Children)
				newItem = append(newItem, item)
			} else if item.Key == config.NavigationKeyCustomerDelivery {
				// if not admin then skip
				if err := util.CheckZadigLicenseFeatureDelivery(); err != nil {
					continue
				} else {
					newItem = append(newItem, item)
				}
			} else if item.Key == config.NavigationKeyDataInsight {
				if err := util.CheckZadigProfessionalLicense(); err != nil {
					continue
				} else {
					newItem = append(newItem, item)
				}
			} else {
				newItem = append(newItem, item)
			}
		}
		return newItem
	}
	newItem := make([]*commonmodels.NavigationItem, 0)
	for _, item := range items {
		if item.Type == config.NavigationItemTypeFolder {
			item.Children = filterNavigationItems(ctx, item.Children)
			newItem = append(newItem, item)
		} else if item.Key == config.NavigationKeyReleasePlan {
			if ctx.Resources.SystemActions.ReleasePlan.View {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyBizCatalog {
			if ctx.Resources.SystemActions.BusinessDirectory.View {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyTemplateLibrary {
			if ctx.Resources.SystemActions.Template.View {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyQualityCenter {
			if ctx.Resources.SystemActions.TestCenter.View {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyArtifactManagement {
			if ctx.Resources.SystemActions.DeliveryCenter.ViewArtifact || ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyResourceConfiguration {
			if ctx.Resources.SystemActions.ClusterManagement.View ||
				ctx.Resources.SystemActions.VMManagement.View ||
				ctx.Resources.SystemActions.RegistryManagement.View ||
				ctx.Resources.SystemActions.S3StorageManagement.View ||
				ctx.Resources.SystemActions.HelmRepoManagement.View ||
				ctx.Resources.SystemActions.DBInstanceManagement.View {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyDataOverview {
			if ctx.Resources.SystemActions.DataCenter.ViewOverView {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyDataInsight {
			if err := util.CheckZadigProfessionalLicense(); err != nil {
				continue
			}
			
			if ctx.Resources.SystemActions.DataCenter.ViewInsight {
				newItem = append(newItem, item)
			}
		} else if item.Key == config.NavigationKeyCustomerDelivery {
			// if not admin then skip
			continue
		} else {
			// no filter
			newItem = append(newItem, item)
		}
	}
	return newItem
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
