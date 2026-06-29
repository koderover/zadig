/*
Copyright 2026 The KodeRover Authors.

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
	"fmt"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetTopology(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := topologyArgsFromQuery(c)
	if args.ProjectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	if !authorizeTopologyView(ctx, args.ProjectName, args.EnvName, args.Production) {
		return
	}

	ctx.Resp, ctx.RespErr = service.BuildEnvironmentTopology(args, ctx.Logger)
}

func GetTopologyNode(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := topologyArgsFromQuery(c)
	args.NodeID = c.Query("nodeId")
	if args.ProjectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	if args.NodeID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("nodeId can't be empty")
		return
	}
	if !authorizeTopologyView(ctx, args.ProjectName, args.EnvName, args.Production) {
		return
	}

	ctx.Resp, ctx.RespErr = service.RefreshTopologyNode(args, ctx.Logger)
}

func GetTopologyDiff(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := topologyArgsFromQuery(c)
	args.NodeID = c.Query("nodeId")
	if args.ProjectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	if args.NodeID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("nodeId can't be empty")
		return
	}
	if !authorizeTopologyView(ctx, args.ProjectName, args.EnvName, args.Production) {
		return
	}

	ctx.Resp, ctx.RespErr = service.GetTopologyDiff(args, ctx.Logger)
}

func SyncTopology(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := topologyArgsFromQuery(c)
	if args.ProjectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	req := new(service.TopologySyncRequest)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if !authorizeTopologySync(ctx, args.ProjectName, args.EnvName, args.Production) {
		return
	}

	detail := fmt.Sprintf("环境名称:%s,服务:%v", args.EnvName, req.ServiceNames)
	detailEn := fmt.Sprintf("Environment Name:%s, Services:%v", args.EnvName, req.ServiceNames)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProjectName, setting.OperationSceneEnv,
		"同步", "环境拓扑", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	ctx.Resp, ctx.RespErr = service.SyncTopology(args, req, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func topologyArgsFromQuery(c *gin.Context) *service.TopologyArgs {
	return &service.TopologyArgs{
		EnvName:     c.Param("name"),
		ProjectName: c.Query("projectName"),
		Production:  c.Query("production") == "true",
		ServiceName: c.Query("serviceName"),
		Refresh:     c.Query("refresh") == "true",
	}
}

func authorizeTopologyView(ctx *internalhandler.Context, projectName, envName string, production bool) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectName]
	if !ok {
		ctx.UnAuthorized = true
		return false
	}
	if production {
		if projectAuth.IsProjectAdmin || projectAuth.ProductionEnv.View {
			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return false
			}
			return true
		}
		permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
		if err == nil && permitted {
			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return false
			}
			return true
		}
		ctx.UnAuthorized = true
		return false
	}
	if projectAuth.IsProjectAdmin || projectAuth.Env.View {
		return true
	}
	permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionView)
	if err != nil || !permitted {
		ctx.UnAuthorized = true
		return false
	}
	return true
}

func authorizeTopologySync(ctx *internalhandler.Context, projectName, envName string, production bool) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectName]
	if !ok {
		ctx.UnAuthorized = true
		return false
	}
	if production {
		if projectAuth.IsProjectAdmin || projectAuth.ProductionEnv.EditConfig {
			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return false
			}
			return true
		}
		permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionEditConfig)
		if err == nil && permitted {
			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return false
			}
			return true
		}
		ctx.UnAuthorized = true
		return false
	}
	if projectAuth.IsProjectAdmin || projectAuth.Env.EditConfig {
		return true
	}
	permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
	if err != nil || !permitted {
		ctx.UnAuthorized = true
		return false
	}
	return true
}
