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

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

// ListKeyVaultItems returns items grouped by group with sensitive values masked
// @Summary List KeyVault Items
// @Description List keyvault items grouped by group with sensitive values masked
// @Tags system
// @Accept json
// @Produce json
// @Param projectName query string false "project name filter"
// @Param isSystem query string false "set to 'true' for system-wide items"
// @Success 200 {object} service.KeyVaultListResponse
// @Router /api/aslan/system/keyvault/items [get]
func ListKeyVaultItems(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	isSystemVariable := c.Query("isSystem")

	if isSystemVariable != "" && projectName != "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("isSystem and projectName cannot be both set")
		return
	}

	ctx.Resp, ctx.RespErr = service.ListKeyVaultItems(projectName, isSystemVariable == "true", ctx.Logger)
}

// GetKeyVaultItem returns a single item with decrypted value (including sensitive)
// @Summary Get KeyVault Item
// @Description Get a single keyvault item with decrypted value
// @Tags system
// @Accept json
// @Produce json
// @Param id path string true "item id"
// @Success 200 {object} commonmodels.KeyVaultItem
// @Router /api/aslan/system/keyvault/items/{id} [get]
func GetKeyVaultItem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id is required")
		return
	}

	// First get the item to check project access
	item, err := service.GetKeyVaultItem(id, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	// Authorization: if projectName is specified, check project access
	if item.ProjectName != "" {
		if !ctx.Resources.IsSystemAdmin {
			projectAuth, ok := ctx.Resources.ProjectAuthInfo[item.ProjectName]
			if !ok {
				ctx.RespErr = e.ErrUnauthorized.AddDesc("unauthorized")
				return
			}
			if !projectAuth.IsProjectAdmin {
				ctx.RespErr = e.ErrUnauthorized.AddDesc("unauthorized")
				return
			}
		}
	} else {
		// System-wide access requires system admin
		if !ctx.Resources.IsSystemAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp = item
}

// CreateKeyVaultItem creates a new keyvault item
// @Summary Create KeyVault Item
// @Description Create a new keyvault item
// @Tags system
// @Accept json
// @Produce json
// @Param body body commonmodels.KeyVaultItem true "keyvault item"
// @Success 200
// @Router /api/aslan/system/keyvault/items [post]
func CreateKeyVaultItem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.KeyVaultItem)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateKeyVaultItem c.GetRawData() err: %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateKeyVaultItem json.Unmarshal err: %s", err)
	}

	detail := fmt.Sprintf("group:%s key:%s project:%s", args.Group, args.Key, args.ProjectName)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "系统设置-密钥库", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// Authorization check
	if args.ProjectName != "" {
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
		}
	} else {
		// System-wide item requires system admin
		if !ctx.Resources.IsSystemAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid keyvault item args")
		return
	}

	if args.Group == "" || args.Key == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("group and key are required")
		return
	}

	args.CreatedBy = ctx.UserName
	args.UpdatedBy = ctx.UserName

	ctx.RespErr = service.CreateKeyVaultItem(args, ctx.Logger)
}

// UpdateKeyVaultItem updates an existing keyvault item
// @Summary Update KeyVault Item
// @Description Update an existing keyvault item
// @Tags system
// @Accept json
// @Produce json
// @Param id path string true "item id"
// @Param body body commonmodels.KeyVaultItem true "keyvault item"
// @Success 200
// @Router /api/aslan/system/keyvault/items/{id} [put]
func UpdateKeyVaultItem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id is required")
		return
	}

	args := new(commonmodels.KeyVaultItem)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateKeyVaultItem c.GetRawData() err: %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateKeyVaultItem json.Unmarshal err: %s", err)
	}

	detail := fmt.Sprintf("id:%s group:%s key:%s", id, args.Group, args.Key)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "更新", "系统设置-密钥库", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// First get the existing item to check project access
	existingItem, err := service.GetKeyVaultItem(id, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	// Authorization check on existing item
	if existingItem.ProjectName != "" {
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[existingItem.ProjectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
		}
	} else {
		// System-wide item requires system admin
		if !ctx.Resources.IsSystemAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	// If changing project, also check access to new project
	if args.ProjectName != existingItem.ProjectName {
		if args.ProjectName != "" {
			if !ctx.Resources.IsSystemAdmin {
				if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
					ctx.UnAuthorized = true
					return
				}
			}
		} else {
			// Moving to system-wide requires system admin
			if !ctx.Resources.IsSystemAdmin {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid keyvault item args")
		return
	}

	if args.Group == "" || args.Key == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("group and key are required")
		return
	}

	args.UpdatedBy = ctx.UserName

	ctx.RespErr = service.UpdateKeyVaultItem(id, args, ctx.Logger)
}

// DeleteKeyVaultItem deletes a keyvault item
// @Summary Delete KeyVault Item
// @Description Delete a keyvault item
// @Tags system
// @Accept json
// @Produce json
// @Param id path string true "item id"
// @Success 200
// @Router /api/aslan/system/keyvault/items/{id} [delete]
func DeleteKeyVaultItem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id is required")
		return
	}

	// First get the item to check project access and log details
	item, err := service.GetKeyVaultItem(id, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("id:%s group:%s key:%s", id, item.Group, item.Key)
	internalhandler.InsertOperationLog(c, ctx.UserName, item.ProjectName, "删除", "系统设置-密钥库", detail, detail, "", types.RequestBodyTypeJSON, ctx.Logger)

	// Authorization check
	if item.ProjectName != "" {
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[item.ProjectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
		}
	} else {
		// System-wide item requires system admin
		if !ctx.Resources.IsSystemAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteKeyVaultItem(id, ctx.Logger)
}

// DeleteKeyVaultGroup deletes all keyvault items in a group
// @Summary Delete KeyVault Group
// @Description Delete all keyvault items in a group (requires system admin for system-wide, project admin for project)
// @Tags system
// @Accept json
// @Produce json
// @Param group path string true "group name"
// @Param projectName query string false "project name filter"
// @Param isSystem query string false "set to 'true' for system-wide items"
// @Success 200
// @Router /api/aslan/system/keyvault/groups/{group} [delete]
func DeleteKeyVaultGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	group := c.Param("group")
	if group == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("group is required")
		return
	}

	projectName := c.Query("projectName")
	isSystemVariable := c.Query("isSystem")

	if isSystemVariable != "" && projectName != "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("isSystem and projectName cannot be both set")
		return
	}

	detail := fmt.Sprintf("group:%s project:%s isSystem:%s", group, projectName, isSystemVariable)

	// Authorization check
	if projectName != "" {
		if !ctx.Resources.IsSystemAdmin {
			projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectName]
			if !ok {
				ctx.UnAuthorized = true
				return
			}
			if !projectAuth.IsProjectAdmin {
				ctx.UnAuthorized = true
				return
			}
		}
		internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "删除", "系统设置-密钥库分组", detail, detail, "", types.RequestBodyTypeJSON, ctx.Logger)
	} else if isSystemVariable == "true" {
		// System-wide item requires system admin
		if !ctx.Resources.IsSystemAdmin {
			ctx.UnAuthorized = true
			return
		}
		internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统设置-密钥库分组", detail, detail, "", types.RequestBodyTypeJSON, ctx.Logger)
	} else {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("either projectName or isSystem must be set")
		return
	}

	ctx.RespErr = service.DeleteKeyVaultGroup(group, projectName, isSystemVariable == "true", ctx.Logger)
}
