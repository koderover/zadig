package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetWorkflowConcurrency(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetWorkflowConcurrency()
}

func UpdateWorkflowConcurrency(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.WorkflowConcurrencySettings)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	// args validation
	if args.BuildConcurrency <= 0 || args.WorkflowConcurrency <= 0 {
		ctx.Err = errors.New("concurrency cannot be less than 1")
		return
	}

	ctx.Err = service.UpdateWorkflowConcurrency(args.WorkflowConcurrency, args.BuildConcurrency)
}
