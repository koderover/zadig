package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// GetHelmVariableFields returns the latest flat Helm values for a service.
// The service is read from the project template, so it does not need to exist
// in the selected environment yet.
func GetHelmVariableFields(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	serviceName := c.Query("serviceName")
	if projectName == "" || serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName and serviceName are required")
		return
	}

	args := new(workflow.HelmVariableFieldsRequest)
	if err := c.ShouldBindYAML(args); err != nil && err != io.EOF {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	production := c.Query("production") == "true"
	envName := c.Query("envName")
	ctx.Resp, ctx.RespErr = workflow.GetHelmVariableFields(projectName, serviceName, envName, production, args, ctx.Logger)
}
