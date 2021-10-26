package handler

import (
	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

type LoadServiceFromYamlTemplateReq struct {
	ServiceName string                 `json:"service_name"`
	ProjectName string                 `json:"project_name"`
	TemplateID  string                 `json:"template_id"`
	Variables   []*svcservice.Variable `json:"variables"`
}

func LoadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(LoadServiceFromYamlTemplateReq)

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = svcservice.LoadServiceFromYamlTemplate(ctx.Username, req.ProjectName, req.ServiceName, req.TemplateID, req.Variables, ctx.Logger)
}

func ReloadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = svcservice.ReloadServiceFromYamlTemplate(ctx.Username, req.ProjectName, req.ServiceName, req.Variables, ctx.Logger)
}
