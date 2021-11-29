package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func DeleteUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	uid := c.Param("id")
	ctx.Resp , ctx.Err = service.DeleteUser(uid,c.Request.Header, c.Request.URL.Query(),ctx.Logger)
}
