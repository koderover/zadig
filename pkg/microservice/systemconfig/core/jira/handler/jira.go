package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func DeleteJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service.DeleteJira(ctx.Logger)
}

func GetJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.GeJira(ctx.Logger)
}

func CreateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateJira(req, ctx.Logger)
}

func UpdateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.UpdateJira(req, ctx.Logger)
}
