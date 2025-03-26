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
	"fmt"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/types"
)

func PatchDebugContainer(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("env")
	podName := c.Param("podName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	debugImage := c.Query("debugImage")
	if debugImage == "" {
		debugImage = types.DebugImage
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "启动调试容器", "环境", envName, "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.DebugPod {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionDebug)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.DebugPod {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionDebug)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.RespErr = service.PatchDebugContainer(c, projectKey, envName, podName, debugImage, production)
}
