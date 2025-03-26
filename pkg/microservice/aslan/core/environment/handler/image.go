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

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func UpdateStatefulSetContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.StatefulSet
	production := c.Query("production") == "true"
	args.Production = production

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateStatefulSetContainerImage c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateStatefulSetContainerImage json.Unmarshal err : %v", err)
		return
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,StatefulSet:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

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

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, args, ctx.Logger)
}

func UpdateDeploymentContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.Deployment

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateDeploymentContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateDeploymentContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,Deployment:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	production := c.Query("production") == "true"
	args.Production = production

	// authorization checks
	permitted := false
	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; ok {
		if production {
			if projectAuthInfo.IsProjectAdmin {
				permitted = true
			} else if projectAuthInfo.ProductionEnv.EditConfig || projectAuthInfo.ProductionEnv.ManagePods {
				permitted = true
			} else {
				collabPermittedConfig, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.ProductionEnvActionEditConfig)
				if err == nil {
					permitted = collabPermittedConfig
				}
				if !permitted {
					collabPermittedManagePod, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.ProductionEnvActionManagePod)
					if err == nil {
						permitted = collabPermittedManagePod
					}
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if projectAuthInfo.IsProjectAdmin {
				permitted = true
			} else if projectAuthInfo.Env.EditConfig || projectAuthInfo.Env.ManagePods {
				permitted = true
			} else {
				collabPermittedConfig, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
				if err == nil {
					permitted = collabPermittedConfig
				}

				if !permitted {
					collabPermittedManagePod, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionManagePod)
					if err == nil {
						permitted = collabPermittedManagePod
					}
				}
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, args, ctx.Logger)
}

func UpdateCronJobContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.CronJob
	production := c.Query("production") == "true"
	args.Production = production

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCronJobContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCronJobContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,CronJob:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

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

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, args, ctx.Logger)
}

type OpenAPIUpdateContainerImageArgs struct {
	Type          string `json:"type"`
	ProductName   string `json:"product_name"`
	EnvName       string `json:"env_name"`
	ServiceName   string `json:"service_name"`
	Name          string `json:"name"`
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`
}

func OpenAPIUpdateDeploymentContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(OpenAPIUpdateContainerImageArgs)
	args.Type = setting.Deployment

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateDeploymentContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateDeploymentContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "OpenAPI-环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,Deployment:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	// authorization checks
	permitted := false
	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; ok {
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		}

		if projectAuthInfo.Env.EditConfig || projectAuthInfo.Env.ManagePods {
			permitted = true
		}

		collabPermittedConfig, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
		if err == nil {
			permitted = collabPermittedConfig
		}

		collabPermittedManagePod, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionManagePod)
		if err == nil {
			permitted = collabPermittedManagePod
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	origArgs := &service.UpdateContainerImageArgs{
		Type:          args.Type,
		ProductName:   args.ProductName,
		EnvName:       args.EnvName,
		ServiceName:   args.ServiceName,
		Name:          args.Name,
		ContainerName: args.ContainerName,
		Image:         args.Image,
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, origArgs, ctx.Logger)
}

func OpenAPIUpdateStatefulSetContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(OpenAPIUpdateContainerImageArgs)
	args.Type = setting.StatefulSet

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateStatefulSetContainerImage c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateStatefulSetContainerImage json.Unmarshal err : %v", err)
		return
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "OpenAPI-环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,StatefulSet:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	origArgs := &service.UpdateContainerImageArgs{
		Type:          args.Type,
		ProductName:   args.ProductName,
		EnvName:       args.EnvName,
		ServiceName:   args.ServiceName,
		Name:          args.Name,
		ContainerName: args.ContainerName,
		Image:         args.Image,
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, origArgs, ctx.Logger)
}

func OpenAPIUpdateCronJobContainerImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(OpenAPIUpdateContainerImageArgs)
	args.Type = setting.CronJob

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateDeploymentContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateDeploymentContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "OpenAPI-环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,CronJob:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	origArgs := &service.UpdateContainerImageArgs{
		Type:          args.Type,
		ProductName:   args.ProductName,
		EnvName:       args.EnvName,
		ServiceName:   args.ServiceName,
		Name:          args.Name,
		ContainerName: args.ContainerName,
		Image:         args.Image,
	}

	ctx.RespErr = service.UpdateContainerImage(ctx.RequestID, ctx.UserName, origArgs, ctx.Logger)
}
