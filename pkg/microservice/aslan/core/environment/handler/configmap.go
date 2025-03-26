/*
Copyright 2021 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListConfigMaps(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	args := &service.ListConfigMapArgs{
		EnvName:     envName,
		ProductName: projectKey,
		ServiceName: c.Query("serviceName"),
		Production:  production,
	}

	ctx.Resp, ctx.RespErr = service.ListConfigMaps(args, ctx.Logger)
}

func RollBackConfigMap(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.RollBackConfigMapArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("RollBackConfigMap c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("RollBackConfigMap json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProductName, setting.OperationSceneEnv, "回滚", "环境-服务-configMap", fmt.Sprintf("环境名称:%s,服务名称:%s", args.EnvName, args.ServiceName), string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	production := c.Query("production") == "true"
	args.Production = production

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].ProductionEnv.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.ProductionEnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].Env.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	if args.SrcConfigName == args.DestinConfigName {
		ctx.RespErr = e.ErrRollBackConfigMap.AddDesc("same source and destination configmap name.")
		return
	}

	ctx.RespErr = service.RollBackConfigMap(args.EnvName, args, ctx.UserName, ctx.UserID, ctx.Logger)
}

func MigrateHistoryConfigMaps(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Query("envName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.MigrateHistoryConfigMaps(envName, projectKey, production, ctx.Logger)
}
