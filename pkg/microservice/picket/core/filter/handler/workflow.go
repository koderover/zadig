package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListTestWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	testName := c.Param("testName")
	ctx.Resp, ctx.Err = service.ListTestWorkflows(testName, c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}

func ListWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListWorkflows(c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}
