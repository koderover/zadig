package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	dockerfileinstructions "github.com/moby/buildkit/frontend/dockerfile/instructions"
	dockerfileparser "github.com/moby/buildkit/frontend/dockerfile/parser"

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
	reader := strings.NewReader(req.Content)
	result, err := dockerfileparser.Parse(reader)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}
	_, _, err = dockerfileinstructions.Parse(result.AST)
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
	reader := strings.NewReader(req.Content)
	result, err := dockerfileparser.Parse(reader)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}
	_, _, err = dockerfileinstructions.Parse(result.AST)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}

	ctx.Err = templateservice.UpdateDockerfileTemplate(c.Param("id"), req, ctx.Logger)
}

type ListDockefileResp struct {
	DockerfileTemplates []*templateservice.DockerfileListObject `json:"dockerfile_template"`
	Total               int                                     `json:"total"`
}

func ListDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	pageSizeString := c.DefaultQuery("page_size", "100")
	pageSize, err := strconv.Atoi(pageSizeString)
	if err != nil {
		ctx.Err = errors.New("invalid page_size query")
		return
	}
	pageNumString := c.DefaultQuery("page_num", "1")
	pageNum, err := strconv.Atoi(pageNumString)
	if err != nil {
		ctx.Err = errors.New("invalid page_num query")
		return
	}

	dockerfileTemplateList, total, err := templateservice.ListDockerfileTemplate(pageNum, pageSize, ctx.Logger)
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

func ValidateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &validateDockerfileTemplateReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
}
