package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

type feature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func GetFeature(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	enabled := service.Features.FeatureEnabled(service.Feature(name))

	ctx.Resp = &feature{
		Name:    c.Param("name"),
		Enabled: enabled,
	}
}

func UpdateOrCreateFeature(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.FeatureReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.UpdateOrCreateFeature(req)
}
