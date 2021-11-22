package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	features := router.Group("features")
	{
		features.PUT("/:name", UpdateOrCreateFeature)
		features.GET("/:name", GetFeature)
	}
}
