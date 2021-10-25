package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.DockerfileTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	// some dockerfile validation stuff
	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}

	ctx.Err = templateservice.CreateDockerfileTemplate(req, ctx.Logger)
}

func UpdateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.DockerfileTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	// some dockerfile validation stuff
	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}

	ctx.Err = templateservice.UpdateDockerfileTemplate(c.Param("id"), req, ctx.Logger)
}

type listDockerfileQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

type ListDockefileResp struct {
	DockerfileTemplates []*templateservice.DockerfileListObject `json:"dockerfile_template"`
	Total               int                                     `json:"total"`
}

func ListDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := listDockerfileQuery{}
	if err := c.ShouldBindQuery(&args); err != nil {
		ctx.Err = err
		return
	}

	dockerfileTemplateList, total, err := templateservice.ListDockerfileTemplate(args.PageNum, args.PageSize, ctx.Logger)
	resp := ListDockefileResp{
		DockerfileTemplates: dockerfileTemplateList,
		Total:               total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetDockerfileTemplateDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetDockerfileTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = templateservice.DeleteDockerfileTemplate(c.Param("id"), ctx.Logger)
}

func GetDockerfileTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetDockerfileTemplateReference(c.Param("id"), ctx.Logger)
}

type validateDockerfileTemplateReq struct {
	Content string `json:"content"`
}

type validateDockerfileTemplateResp struct {
	Error string `json:"error"`
}

func ValidateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &validateDockerfileTemplateReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	ctx.Resp = &validateDockerfileTemplateResp{Error: ""}
	if err != nil {
		ctx.Resp = &validateDockerfileTemplateResp{Error: err.Error()}
	}
}
