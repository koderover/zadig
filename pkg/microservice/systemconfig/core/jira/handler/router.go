package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	jira := router.Group("jira")
	{
		jira.GET("", GetJira)
		jira.POST("", CreateJira)
		jira.PATCH("", UpdateJira)
		jira.DELETE("", DeleteJira)
	}

}
