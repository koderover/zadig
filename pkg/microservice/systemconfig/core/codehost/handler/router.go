package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	codehost := router.Group("codehost")
	{
		codehost.GET("", ListCodeHost)
		codehost.DELETE("/:id", DeleteCodeHost)
		codehost.POST("", CreateCodeHost)
	}
}
