/*
Copyright 2022 The KodeRover Authors.

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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/types"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func DeleteCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	commonEnvCfgType := c.Query("commonEnvCfgType")
	objectName := c.Param("objectName")
	if envName == "" || projectKey == "" || objectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("param envName or projectName or objectName is invalid")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "删除", "环境配置", fmt.Sprintf("%s:%s:%s", envName, commonEnvCfgType, objectName), "", ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = service.DeleteCommonEnvCfg(envName, projectKey, objectName, config.CommonEnvCfgType(commonEnvCfgType), ctx.Logger)
}

func DeleteProductionCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	commonEnvCfgType := c.Query("commonEnvCfgType")
	objectName := c.Param("objectName")
	if envName == "" || projectKey == "" || objectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("param envName or projectName or objectName is invalid")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "删除", "环境配置", fmt.Sprintf("%s:%s:%s", envName, commonEnvCfgType, objectName), "", ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = service.DeleteCommonEnvCfg(envName, projectKey, objectName, config.CommonEnvCfgType(commonEnvCfgType), ctx.Logger)
}

func CreateCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateCommonEnvCfg c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateCommonEnvCfg json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "新建", "环境配置", fmt.Sprintf("%s:%s", args.EnvName, args.CommonEnvCfgType), string(data), ctx.Logger, c.Param("name"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if args.YamlData == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}
	args.EnvName = envName
	args.ProductName = projectKey
	ctx.Err = service.CreateCommonEnvCfg(args, ctx.UserName, ctx.Logger)
}

func CreateProductionCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateCommonEnvCfg c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateCommonEnvCfg json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "新建", "环境配置", fmt.Sprintf("%s:%s", args.EnvName, args.CommonEnvCfgType), string(data), ctx.Logger, c.Param("name"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if args.YamlData == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}
	args.EnvName = envName
	args.ProductName = projectKey
	ctx.Err = service.CreateCommonEnvCfg(args, ctx.UserName, ctx.Logger)
}

func UpdateCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCommonEnvCfg c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCommonEnvCfg json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "更新", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), ctx.Logger, c.Param("name"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if len(args.YamlData) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("yaml info can't be nil")
		return
	}
	args.EnvName = envName
	args.ProductName = projectKey
	isRollBack := false
	if len(c.Query("rollback")) > 0 {
		isRollBack, err = strconv.ParseBool(c.Query("rollback"))
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddErr(err)
			return
		}
	}

	ctx.Err = service.UpdateCommonEnvCfg(args, ctx.UserName, isRollBack, ctx.Logger)
}

func UpdateProductionCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCommonEnvCfg c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCommonEnvCfg json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "更新", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), ctx.Logger, c.Param("name"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if len(args.YamlData) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("yaml info can't be nil")
		return
	}
	args.EnvName = envName
	args.ProductName = projectKey
	isRollBack := false
	if len(c.Query("rollback")) > 0 {
		isRollBack, err = strconv.ParseBool(c.Query("rollback"))
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddErr(err)
			return
		}
	}

	ctx.Err = service.UpdateCommonEnvCfg(args, ctx.UserName, isRollBack, ctx.Logger)
}

func ListCommonEnvCfgHistory(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	args := new(service.ListCommonEnvCfgHistoryArgs)
	args.EnvName = envName
	args.ProjectName = projectKey
	args.CommonEnvCfgType = config.CommonEnvCfgType(c.Query("commonEnvCfgType"))
	args.Name = c.Param("objectName")

	ctx.Resp, ctx.Err = service.ListEnvResourceHistory(args, ctx.Logger)
}

func ListProductionCommonEnvCfgHistory(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(service.ListCommonEnvCfgHistoryArgs)
	args.EnvName = envName
	args.ProjectName = projectKey
	args.CommonEnvCfgType = config.CommonEnvCfgType(c.Query("commonEnvCfgType"))
	args.Name = c.Param("objectName")

	ctx.Resp, ctx.Err = service.ListEnvResourceHistory(args, ctx.Logger)
}

func ListLatestEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.ListCommonEnvCfgHistoryArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Env.View {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProjectName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.Err = service.ListLatestEnvResources(args, ctx.Logger)
}

func SyncEnvResource(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	args := &service.SyncEnvResourceArg{
		EnvName:     envName,
		ProductName: projectKey,
		Name:        c.Param("objectName"),
		Type:        c.Param("type"),
	}
	ctx.Err = service.SyncEnvResource(args, ctx.Logger)
}
