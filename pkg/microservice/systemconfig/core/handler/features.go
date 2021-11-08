package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/service/featuregates"
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
	enabled := featuregates.Features.FeatureEnabled(featuregates.Feature(name))

	ctx.Resp = &feature{
		Name:    c.Param("name"),
		Enabled: enabled,
	}
}
