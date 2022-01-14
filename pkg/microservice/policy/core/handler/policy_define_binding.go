package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreatePolicyDefineBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	args := make([]*service.PolicyDefineBinding, 0)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.CreatePolicyDefineBindings(projectName, args, ctx.Logger)

}
