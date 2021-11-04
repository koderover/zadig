package handler

import (
	"github.com/gin-gonic/gin"

	service2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/service"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func DeleteJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service2.DeleteJira(ctx.Logger)
}

func GetJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service2.GeJira(ctx.Logger)
}

func CreateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.CreateJira(req, ctx.Logger)
}

func UpdateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.UpdateJira(req, ctx.Logger)
}
