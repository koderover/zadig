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
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	buildhandler "github.com/koderover/zadig/lib/microservice/aslan/core/build/handler"
	codehosthandler "github.com/koderover/zadig/lib/microservice/aslan/core/code/handler"
	commonhandler "github.com/koderover/zadig/lib/microservice/aslan/core/common/handler"
	cronhandler "github.com/koderover/zadig/lib/microservice/aslan/core/cron/handler"
	deliveryhandler "github.com/koderover/zadig/lib/microservice/aslan/core/delivery/handler"
	enterprisehandler "github.com/koderover/zadig/lib/microservice/aslan/core/enterprise/handler"
	environmenthandler "github.com/koderover/zadig/lib/microservice/aslan/core/environment/handler"
	loghandler "github.com/koderover/zadig/lib/microservice/aslan/core/log/handler"
	projecthandler "github.com/koderover/zadig/lib/microservice/aslan/core/project/handler"
	servicehandler "github.com/koderover/zadig/lib/microservice/aslan/core/service/handler"
	settinghandler "github.com/koderover/zadig/lib/microservice/aslan/core/setting/handler"
	systemhandler "github.com/koderover/zadig/lib/microservice/aslan/core/system/handler"
	workflowhandler "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/handler"
	"github.com/koderover/zadig/lib/microservice/aslan/middleware"
)

func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
	store := sessions.NewCookieStore([]byte("C12f4d5957k345e9g6e86c117e1bfu5kndjevf8u"))
	cookieOption := sessions.Options{
		Path:     "/",
		HttpOnly: true,
	}
	store.Options(cookieOption)
	router.Use(sessions.Sessions("ASLAN", store))
	Auth := middleware.Auth()
	// ---------------------------------------------------------------------------------------
	// 对外公共接口
	// ---------------------------------------------------------------------------------------
	public := router.Group("/api")
	{
		// 内部路由重定向。接口路由更新后，需要保证旧路由可用，否则github、gitlab的配置需要手动更新。
		public.POST("/ci/webhook", func(c *gin.Context) {
			c.Request.URL.Path = "/api/workflow/webhook/ci/webhook"
			s.HandleContext(c)
		})
		public.POST("/githubWebHook", func(c *gin.Context) {
			c.Request.URL.Path = "/api/workflow/webhook/githubWebHook"
			s.HandleContext(c)
		})
		public.POST("/gitlabhook", func(c *gin.Context) {
			c.Request.URL.Path = "/api/workflow/webhook/gitlabhook"
			s.HandleContext(c)
		})
		public.POST("/gerritHook", func(c *gin.Context) {
			c.Request.URL.Path = "/api/workflow/webhook/gerritHook"
			s.HandleContext(c)
		})
		public.GET("/health", commonhandler.Health)
	}

	router.GET("/api/kodespace/downloadUrl", Auth, commonhandler.GetToolDownloadURL)

	jwt := router.Group("/api/token", Auth)
	{
		jwt.GET("", commonhandler.GetToken)
	}
	for name, r := range map[string]injector{
		"/api/project":     new(projecthandler.Router),
		"/api/code":        new(codehosthandler.Router),
		"/api/system":      new(systemhandler.Router),
		"/api/service":     new(servicehandler.Router),
		"/api/setting":     new(settinghandler.Router),
		"/api/environment": new(environmenthandler.Router),
		"/api/cron":        new(cronhandler.Router),
		"/api/workflow":    new(workflowhandler.Router),
		"/api/build":       new(buildhandler.Router),
		"/api/enterprise":  new(enterprisehandler.Router),
		"/api/delivery":    new(deliveryhandler.Router),
		"/api/logs":        new(loghandler.Router),
	} {
		r.Inject(router.Group(name))
	}
}

type injector interface {
	Inject(router *gin.RouterGroup)
}
