/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import "github.com/gin-gonic/gin"

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	v1 := router.Group("v1")
	{
		sprintTemplate := v1.Group("sprint_template")
		{
			sprintTemplate.GET("", ListSprintTemplates)
			sprintTemplate.POST("", CreateSprintTemplate)
			sprintTemplate.GET("/default", GetDefaultSprintTemplates)
			sprintTemplate.GET("/:id", GetSprintTemplate)
			sprintTemplate.DELETE("/:id", DeleteSprintTemplate)

			sprintTemplate.POST("/:id/stage", AddSprintTemplateStage)
			sprintTemplate.PUT("/:id/stage/:stageID/name", UpdateSprintTemplateStageName)
			sprintTemplate.PUT("/:id/stage/:stageID/workflows", UpdateSprintTemplateStageWorkflows)
			sprintTemplate.DELETE("/:id/stage/:stageID", DeleteSprintTemplateStage)
		}

		sprint := v1.Group("sprint")
		{
			sprint.GET("", ListSprint)
			sprint.POST("", CreateSprint)
			sprint.GET("/:id", GetSprint)
			sprint.PUT("/:id/name", UpdateSprintName)
			sprint.PUT("/:id/archive", ArchiveSprint)
			sprint.PUT("/:id/activate", ActivateArchivedSprint)
			sprint.DELETE("/:id", DeleteSprint)
		}

		sprintWorkItem := v1.Group("sprint_workitem")
		{
			sprintWorkItem.POST("", CreateSprintWorkItem)
			sprintWorkItem.GET("/:id", GetSprintWorkItem)
			sprintWorkItem.PUT("/:id/title", UpdateSprintWorkItemTitle)
			sprintWorkItem.PUT("/:id/desc", UpdateSprintWorkItemDescription)
			sprintWorkItem.PUT("/:id/owner", UpdateSprintWorkItemOwner)
			sprintWorkItem.PUT("/:id/move", MoveSprintWorkItem)
			sprintWorkItem.DELETE("/:id", DeleteSprintWorkItem)

			sprintWorkItem.GET("/:id/task", ListSprintWorkItemTask)
			sprintWorkItem.POST("/:id/task/exec", ExecSprintWorkItemWorkflow)
			sprintWorkItem.POST("/:id/task/clone", CloneSprintWorkItemTask)
		}

	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
}
