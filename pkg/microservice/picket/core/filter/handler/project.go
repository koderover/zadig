package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

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
	if err := c.ShouldBindBodyWith(args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err).AddDesc("invalid CreateProjectReq")
		return
	}
	var body []byte
	if cb, ok := c.Get(gin.BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}

	ctx.Resp, ctx.Err = service.CreateProject(c.Request.Header, body, args.ProductName, args.Public, ctx.Logger)
}

type UpdateProjectReq struct {
	Public      bool   `json:"public"`
	ProductName string `json:"product_name"`
}

func UpdateProject(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(UpdateProjectReq)
	if err := c.ShouldBindBodyWith(args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err).AddDesc("invalid UpdateProject")
		return
	}
	var body []byte
	if cb, ok := c.Get(gin.BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}
	ctx.Resp, ctx.Err = service.UpdateProject(c.Request.Header, body, c.Request.URL.Query(), args.ProductName, args.Public, ctx.Logger)
}
