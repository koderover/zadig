package handler

import (
	"github.com/gin-gonic/gin"

	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.CreateYamlTemplate(req, ctx.Logger)
}

func UpdateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.UpdateYamlTemplate(c.Param("id"), req, ctx.Logger)
}

type listYamlQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

type ListYamlResp struct {
	SystemVariables []*templateservice.Variable       `json:"system_variables"`
	YamlTemplates   []*templateservice.YamlListObject `json:"yaml_template"`
	Total           int                               `json:"total"`
}

func ListYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listYamlQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	systemVariables := templateservice.GetSystemDefaultVariables()
	YamlTemplateList, total, err := templateservice.ListYamlTemplate(args.PageNum, args.PageSize, ctx.Logger)
	resp := ListYamlResp{
		SystemVariables: systemVariables,
		YamlTemplates:   YamlTemplateList,
		Total:           total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetYamlTemplateDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = templateservice.DeleteYamlTemplate(c.Param("id"), ctx.Logger)
}

func GetYamlTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateReference(c.Param("id"), ctx.Logger)
}

type updateYamlTemplateVariablesReq struct {
	Variables []*templateservice.Variable `json:"variables"`
}

func UpdateYamlTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &updateYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.UpdateYamlTemplateVariables(c.Param("id"), req.Variables, ctx.Logger)
}
