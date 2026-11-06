package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
)

type Router struct{ Service *service.AgentService }

func (r *Router) Inject(router *gin.RouterGroup) {
	a := &API{Agent: r.Service}
	api := router.Group("/api/halman", a.authenticate)
	api.GET("/skills", a.requireSkillAdmin, a.listSkills)
	api.PUT("/skills/:skill_id", a.requireSkillAdmin, a.updateSkill)
	api.POST("/conversations", a.requireEnabled, a.createConversation)
	api.GET("/conversations", a.listConversations)
	api.DELETE("/conversations/:id", a.deleteConversation)
	api.GET("/conversations/:id/messages", a.messages)
	api.POST("/conversations/:id/runs", a.requireEnabled, a.agui)
	api.GET("/runs/:id", a.getRun)
	api.POST("/runs/:id/cancel", a.cancelRun)
	api.POST("/runs/:id/confirmation", a.confirm)
	api.POST("/runs/:id/feedback", a.feedback)
	api.POST("/runs/:id/regenerate", a.requireEnabled, a.regenerate)
	api.POST("/runs/:id/retry", a.requireEnabled, a.retry)
	api.POST("/runs/:id/reconcile", a.reconcile)
	api.GET("/recommendations", a.requireEnabled, a.recommendations)
	api.GET("/runs/:id/events", a.events)
}
