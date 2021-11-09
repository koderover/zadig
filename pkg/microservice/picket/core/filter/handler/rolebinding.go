package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.ListRoleBindings(c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}
