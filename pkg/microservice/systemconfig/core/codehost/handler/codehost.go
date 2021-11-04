package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/service"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateCodehost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	rep := new(models.CodeHost)
	if err := c.ShouldBindJSON(rep); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateCodehost(rep, ctx.Logger)
}

func ListCodehost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.FindCodehost(ctx.Logger)
}
