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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get Deployable Envs
// @Description Get Deployable Envs
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Success 200 			{object}  	service.DeployableEnvResp
// @Router /api/aslan/service/services/{name}/environments/deployable [get]
func GetDeployableEnvs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to get authorization info: %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = fmt.Errorf("projectName is required")
		return
	}

	production := c.Query("production") == "true"
	hasPermission := ctx.Resources.IsSystemAdmin
	if production {
		if projectInfo := ctx.Resources.ProjectAuthInfo[projectName]; projectInfo != nil {
			if projectInfo.IsProjectAdmin ||
				(projectInfo.ProductionEnv != nil && projectInfo.ProductionEnv.View) {
				hasPermission = true
			}
		}
	} else {
		if projectInfo := ctx.Resources.ProjectAuthInfo[projectName]; projectInfo != nil {
			if projectInfo.IsProjectAdmin ||
				(projectInfo.Env != nil && projectInfo.Env.View) {
				hasPermission = true
			}
		}
	}

	envFilter := sets.NewString()
	if !hasPermission {
		permittedEnv, err := internalhandler.ListCollaborationEnvironmentsPermission(ctx.UserID, projectName)
		if err != nil {
			ctx.RespErr = fmt.Errorf("failed to get environment permissions: %s", err)
			ctx.UnAuthorized = true
			return
		}
		if permittedEnv == nil || len(permittedEnv.ReadEnvList) == 0 {
			ctx.UnAuthorized = true
			return
		}
		envFilter.Insert(permittedEnv.ReadEnvList...)
	}

	if production {
		err := commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	resp, err := service.GetDeployableEnvs(c.Param("name"), projectName, production)
	if err != nil {
		ctx.RespErr = err
		return
	}
	if !hasPermission {
		envs := make([]*service.DeployableEnv, 0)
		for _, env := range resp.Envs {
			if envFilter.Has(env.EnvName) {
				envs = append(envs, env)
			}
		}
		resp.Envs = envs
	}
	ctx.Resp = resp
}

// GetKubeWorkloads api used to force user to have get environments privilege to use, now it is removed.
// Revert if necessary
func GetKubeWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.GetKubeWorkloads(c.Query("namespace"), c.Query("cluster_id"), ctx.Logger)
}

func LoadKubeWorkloadsYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.LoadKubeWorkloadsYamlReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("copyHelmProduct c.GetRawData() err : %s", err)
	} else if err = json.Unmarshal(data, &args); err != nil {
		ctx.Logger.Errorf("copyHelmProduct json.Unmarshal err : %s", err)
	}
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	serviceNames := make([]string, 0)
	for _, svc := range args.Services {
		serviceNames = append(serviceNames, svc.Name)
	}

	production := c.Query("production") == "true"
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	detail := fmt.Sprintf("服务名称:%s", strings.Join(serviceNames, ","))
	detailEn := fmt.Sprintf("Service Name: %s", strings.Join(serviceNames, ","))
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", function, detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.LoadKubeWorkloadsYaml(ctx.UserName, args, false, production, ctx.Logger)
}
