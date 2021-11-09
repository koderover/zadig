package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	codehost := router.Group("codehosts")
	{
		codehost.GET("/callback", Callback)
		codehost.GET("", ListCodeHost)
		codehost.DELETE("/:id", DeleteCodeHost)
		codehost.POST("", CreateCodeHost)
		codehost.PATCH("/:id", UpdateCodeHost)
		codehost.GET("/:id", GetCodeHost)
		codehost.GET("/:id/auth", AuthCodeHost)
	}
}
