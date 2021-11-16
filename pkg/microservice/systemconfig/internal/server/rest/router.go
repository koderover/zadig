/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"strconv"

	"github.com/gin-gonic/gin"
	//codehosthandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/handler"
	//emailHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/handler"
	//"github.com/koderover/zadig/pkg/microservice/systemconfig/core/handler"
	//jiraHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/handler"
	//codehosthandler "github.com/koderover/zadig/pkg/microservice/aslan/core/code/handler"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/internal/service"
)

func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
	for _, r := range []injector{
		//new(handler.Router),
		//new(emailHandler.Router),
		//new(jiraHandler.Router),
		&Router{engine: s},
	} {
		r.Inject(router.Group("/api/v1", GetCodeHost(s.CodeHostService)))
	}
}

type injector interface {
	Inject(router *gin.RouterGroup)
}

type Router struct {
	engine *engine
}

func (r *Router) Inject(router *gin.RouterGroup) {
	codehost := router.Group("codehosts")
	{
		//codehost.GET("/callback", Callback)
		//codehost.GET("", ListCodeHost)
		//codehost.DELETE("/:id", DeleteCodeHost)
		//codehost.POST("", CreateCodeHost)
		//codehost.PATCH("/:id", UpdateCodeHost)
		codehost.GET("/:id", GetCodeHost(r.engine.CodeHostService))
		//codehost.GET("/:id/auth", AuthCodeHost)
	}
}

func GetCodeHost(s *service.CodeHostService) func(c *gin.Context) {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			return
		}
		r, _ := s.GetCodeHost(c, id)
		c.JSON(200, r)
	}
}
