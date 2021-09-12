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
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"

	buildhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/build/handler"
	codehosthandler "github.com/koderover/zadig/pkg/microservice/aslan/core/code/handler"
	commonhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/common/handler"
	cronhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/cron/handler"
	deliveryhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/handler"
	environmenthandler "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/handler"
	loghandler "github.com/koderover/zadig/pkg/microservice/aslan/core/log/handler"
	multiclusterhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/multicluster/handler"
	projecthandler "github.com/koderover/zadig/pkg/microservice/aslan/core/project/handler"
	servicehandler "github.com/koderover/zadig/pkg/microservice/aslan/core/service/handler"
	settinghandler "github.com/koderover/zadig/pkg/microservice/aslan/core/setting/handler"
	systemhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/system/handler"
	templatehandler "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/handler"
	workflowhandler "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/handler"
	testinghandler "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/handler"
	gin2 "github.com/koderover/zadig/pkg/middleware/gin"

	// Note: have to load docs for swagger to work. See https://blog.csdn.net/weixin_43249914/article/details/103035711
	_ "github.com/koderover/zadig/pkg/microservice/aslan/server/rest/doc"
)

// @title Zadig aslan service REST APIs
// @version 1.0
// @description The API doc is targeting for Zadig developers rather than Zadig users.
// @description The majority of these APIs are not designed for public use, there is no guarantee on version compatibility.
// @description Please reach out to contact@koderover.com before you decide to depend on these APIs directly.
// @contact.email contact@koderover.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /api/aslan
func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
	store := cookie.NewStore([]byte("C12f4d5957k345e9g6e86c117e1bfu5kndjevf8u"))
	cookieOption := sessions.Options{
		Path:     "/",
		HttpOnly: true,
	}
	store.Options(cookieOption)
	router.Use(sessions.Sessions("ASLAN", store))
	Auth := gin2.Auth()
	// ---------------------------------------------------------------------------------------
	// 对外公共接口
	// ---------------------------------------------------------------------------------------
	public := router.Group("/api")
	{
		public.POST("/webhook", func(c *gin.Context) {
			c.Request.URL.Path = "/api/workflow/webhook"
			s.HandleContext(c)
		})
		public.GET("/health", commonhandler.Health)
	}

	// no auth required, should not be exposed via poetry-api-proxy or will fail
	router.GET("/api/hub/connect", multiclusterhandler.ClusterConnectFromAgent)

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
		"/api/delivery":    new(deliveryhandler.Router),
		"/api/logs":        new(loghandler.Router),
		"/api/testing":     new(testinghandler.Router),
		"/api/cluster":     new(multiclusterhandler.Router),
		"/api/template":    new(templatehandler.Router),
	} {
		r.Inject(router.Group(name))
	}

	router.GET("/api/apidocs/*any", ginswagger.WrapHandler(swaggerfiles.Handler))
}

type injector interface {
	Inject(router *gin.RouterGroup)
}
