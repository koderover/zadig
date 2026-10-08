package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// GetHelmVariableFields returns the latest flat Helm values for a service.
// The service is read from the project template, so it does not need to exist
// in the selected environment yet.
func GetHelmVariableFields(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	serviceName := c.Query("serviceName")
	if projectName == "" || serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName and serviceName are required")
		return
	}

	production := c.Query("production") == "true"
	envName := c.Query("envName")

	if !ctx.Resources.IsSystemAdmin {
		projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectName]
		if !ok {
			ctx.UnAuthorized = true
			return
		}

		if !projectAuth.IsProjectAdmin {
			if envName == "" {
				allowed := projectAuth.Service.View
				if production {
					allowed = projectAuth.ProductionService.View
				}
				if !allowed {
					ctx.UnAuthorized = true
					return
				}
			} else {
				allowed := projectAuth.Env.View
				action := types.EnvActionView
				if production {
					allowed = projectAuth.ProductionEnv.View
					action = types.ProductionEnvActionView
				}
				if !allowed {
					allowed, err = internalhandler.GetCollaborationModePermission(
						ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, action,
					)
					if err != nil || !allowed {
						ctx.UnAuthorized = true
						return
					}
				}
			}
		}
	}

	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = workflow.GetHelmVariableFields(projectName, serviceName, envName, production, ctx.Logger)
}
