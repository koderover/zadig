package handler

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {

	dev := router.Group("")
	{
		dev.POST("/workflowTask/create", CreateWorkflowTask)
		dev.DELETE("workflowTask/id/:id/pipelines/:name/cancel", CancelWorkflowTask)
		dev.POST("/workflowTask/id/:id/pipelines/:name/restart", RestartWorkflowTask)
		dev.GET("/workflowTask", ListWorkflowTask)
		dev.GET("/dc/releases", ListDelivery)
	}
}
