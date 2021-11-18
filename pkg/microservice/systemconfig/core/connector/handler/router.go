package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	connector := router.Group("connectors")
	{
		connector.POST("", CreateConnector)
		connector.GET("", ListConnectors)
		connector.GET("/:id", GetConnector)
		connector.PUT("/:id", UpdateConnector)
		connector.DELETE("/:id", DeleteConnector)
	}
}
