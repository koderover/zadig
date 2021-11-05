package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	emails := router.Group("emails")
	{
		emails.GET("/host", GetEmailHost)
		emails.POST("/host", CreateEmailHost)
		emails.PATCH("/host", UpdateEmailHost)
		emails.DELETE("/host", DeleteEmailHost)

		emails.GET("/internal/host", InternalGetEmailHost)

		emails.GET("/service", GetEmailService)
		emails.POST("/service", CreateEmailService)
		emails.PATCH("/service", UpdateEmailService)
		emails.DELETE("/service", DeleteEmailService)
	}
}
