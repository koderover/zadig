package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProjects(c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}

type CreateProjectReq struct {
	Public      bool   `json:"public"`
	ProductName string `json:"product_name"`
}

func CreateProject(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(CreateProjectReq)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err).AddDesc("invalid CreateProjectReq")
		return
	}
	body, _ := c.GetRawData()

	ctx.Resp, ctx.Err = service.CreateProject(c.Request.Header, body, args.ProductName, args.Public, ctx.Logger)
}
