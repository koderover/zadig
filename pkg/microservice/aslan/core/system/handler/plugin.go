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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListPlugins(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListPlugins(ctx.Logger)
}

func CreatePlugin(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	// accept multipart form (fields + file)
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	contentType := c.Request.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("expect multipart/form-data with fields and file")
		return
	}

	if err := c.Request.ParseMultipartForm(50 * 1024 * 1024); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid multipart form")
		return
	}

	args := new(commonmodels.Plugin)
	args.Name = c.PostForm("name")
	if args.Name == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("name is required")
		return
	}
	args.Identifier = c.PostForm("identifier")
	if idxStr := c.PostForm("index"); idxStr != "" {
		if parsed, err := strconv.Atoi(idxStr); err == nil {
			args.Index = parsed
		}
	}
	args.Type = c.PostForm("type")
	args.Description = c.PostForm("description")
	args.Route = c.PostForm("route")
	args.Enabled = c.PostForm("enabled") == "true"

	// parse filters if provided
	if filtersStr := c.PostForm("filters"); filtersStr != "" {
		var filters []*commonmodels.PluginFilter
		if err := json.Unmarshal([]byte(filtersStr), &filters); err != nil {
			ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid filters format, should be JSON array")
			return
		}
		args.Filters = filters
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("file is required")
		return
	}
	defer file.Close()

	// Service will upload and analyze the file; and persist the plugin
	ctx.RespErr = service.CreatePluginWithFile(ctx.UserName, args, header, file, ctx.Logger)

	meta := map[string]string{
		"name":       args.Name,
		"type":       args.Type,
		"storage_id": args.StorageID,
	}
	metaBytes, _ := json.Marshal(meta)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统设置-插件管理", fmt.Sprintf("name:%s", args.Name), string(metaBytes), types.RequestBodyTypeJSON, ctx.Logger)
}

func UpdatePlugin(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	contentType := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		// multipart update: fields + optional file
		if err := c.Request.ParseMultipartForm(50 * 1024 * 1024); err != nil {
			ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid multipart form")
			return
		}
		args := new(commonmodels.Plugin)
		args.Name = c.PostForm("name")
		args.Identifier = c.PostForm("identifier")
		if idxStr := c.PostForm("index"); idxStr != "" {
			if parsed, err := strconv.Atoi(idxStr); err == nil {
				args.Index = parsed
			}
		}
		args.Type = c.PostForm("type")
		args.Description = c.PostForm("description")
		args.Route = c.PostForm("route")
		args.Enabled = c.PostForm("enabled") == "true"

		// parse filters if provided
		if filtersStr := c.PostForm("filters"); filtersStr != "" {
			var filters []*commonmodels.PluginFilter
			if err := json.Unmarshal([]byte(filtersStr), &filters); err != nil {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid filters format, should be JSON array")
				return
			}
			args.Filters = filters
		}

		// try file
		file, header, err := c.Request.FormFile("file")
		if err == nil {
			defer file.Close()
			ctx.RespErr = service.UpdatePluginWithFile(ctx.UserName, c.Param("id"), args, header, file, ctx.Logger)
		} else {
			ctx.RespErr = fmt.Errorf("file is required")
		}
		op := map[string]string{"id": c.Param("id"), "name": args.Name, "type": args.Type}
		metaBytes, _ := json.Marshal(op)
		internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统设置-插件管理", fmt.Sprintf("id:%s name:%s", c.Param("id"), args.Name), string(metaBytes), types.RequestBodyTypeJSON, ctx.Logger)
		return
	} else {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid content type: %s", contentType))
	}
}

func DeletePlugin(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统设置-插件管理", fmt.Sprintf("id:%s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger)
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	ctx.RespErr = service.DeletePlugin(c.Param("id"), ctx.Logger)
}

// GetPluginFile downloads the file of the plugin by id
// NOTE: service logic is intentionally left to you, see the TODO in service.GetPluginFilePath
func GetPluginFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	id := c.Param("id")
	if id == "" {
		c.JSON(e.ErrorMessage(e.ErrInvalidParam.AddDesc("empty plugin id")))
		c.Abort()
		return
	}
	absolutePath, fileName, err := service.GetPluginFile(id, ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.File(absolutePath)
}
