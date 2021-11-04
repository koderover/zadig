package handler

import (
	"github.com/gin-gonic/gin"

	models2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/models"
	service2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service2.GetEmailHost(ctx.Logger)
}

func CreateEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models2.EmailHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.CreateEmailHost(req, ctx.Logger)
}

func UpdateEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models2.EmailHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.UpdateEmailHost(req, ctx.Logger)
}

func DeleteEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service2.DeleteEmailHost(ctx.Logger)
}

////

func GetEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service2.GetEmailService(ctx.Logger)
}

func CreateEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models2.EmailService)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.CreateEmailService(req, ctx.Logger)
}

func UpdateEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models2.EmailService)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service2.UpdateEmailService(req, ctx.Logger)
}

func DeleteEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service2.DeleteEmailService(ctx.Logger)
}
