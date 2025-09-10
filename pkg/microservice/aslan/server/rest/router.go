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
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"

	cachehandler "github.com/koderover/zadig/v2/pkg/handler/cache"
	buildhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/build/handler"
	codehosthandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/handler"
	collaborationhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/handler"
	commonhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/handler"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	cronhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/cron/handler"
	deliveryhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/handler"
	environmenthandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/handler"
	loghandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/handler"
	multiclusterhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/handler"
	clusterservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	projecthandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/handler"
	releaseplanhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/release_plan/handler"
	servicehandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/handler"
	sprintmanagementhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/sprint_management/handler"
	stathandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/handler"
	systemhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/handler"
	templatehandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/handler"
	tickethandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/ticket/handler"
	vmhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/vm/handler"
	workflowhandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/handler"
	testinghandler "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/handler"
	evaluationhandler "github.com/koderover/zadig/v2/pkg/microservice/picket/core/evaluation/handler"
	filterhandler "github.com/koderover/zadig/v2/pkg/microservice/picket/core/filter/handler"
	publichandler "github.com/koderover/zadig/v2/pkg/microservice/picket/core/public/handler"
	podexecservice "github.com/koderover/zadig/v2/pkg/microservice/podexec/core/service"
	configcodehostHandler "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/handler"
	connectorHandler "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/connector/handler"
	emailHandler "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/email/handler"
	featuresHandler "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/features/handler"
	"github.com/koderover/zadig/v2/pkg/tool/metrics"
	// Note: have to load docs for swagger to work. See https://blog.csdn.net/weixin_43249914/article/details/103035711
	// _ "github.com/koderover/zadig/v2/pkg/microservice/aslan/server/rest/doc"
)

func init() {
	// initialization for prometheus metrics
	metrics.Metrics = prometheus.NewRegistry()

	metrics.Metrics.MustRegister(metrics.RunningWorkflows)
	metrics.Metrics.MustRegister(metrics.PendingWorkflows)
	metrics.Metrics.MustRegister(metrics.RequestTotal)
	metrics.Metrics.MustRegister(metrics.CPU)
	metrics.Metrics.MustRegister(metrics.Memory)
	metrics.Metrics.MustRegister(metrics.CPUPercentage)
	metrics.Metrics.MustRegister(metrics.MemoryPercentage)
	metrics.Metrics.MustRegister(metrics.Healthy)
	metrics.Metrics.MustRegister(metrics.Cluster)
	metrics.Metrics.MustRegister(metrics.ResponseTime)

	metrics.UpdatePodMetrics()
}

// @title Zadig aslan service REST APIs
// @version 1.0
// @description The API doc is targeting for Zadig developers rather than Zadig users.
// @description The majority of these APIs are not designed for public use, there is no guarantee on version compatibility.
// @description Please reach out to contact@koderover.com before you decide to depend on these APIs directly.
// @contact.email contact@koderover.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
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
		public.POST("/callback", commonhandler.HandleCallback)
		public.POST("/callback/release_plan", releaseplanhandler.ReleasePlanHookCallback)
	}

	for name, r := range map[string]injector{
		"/openapi/statistics":   new(stathandler.OpenAPIRouter),
		"/openapi/projects":     new(projecthandler.OpenAPIRouter),
		"/openapi/system":       new(systemhandler.OpenAPIRouter),
		"/openapi/workflows":    new(workflowhandler.OpenAPIRouter),
		"/openapi/environments": new(environmenthandler.OpenAPIRouter),
		"/openapi/quality":      new(testinghandler.QualityRouter),
		"/openapi/build":        new(buildhandler.OpenAPIRouter),
		"/openapi/service":      new(servicehandler.OpenAPIRouter),
		"/openapi/release_plan": new(releaseplanhandler.OpenAPIRouter),
		"/openapi/delivery":     new(deliveryhandler.OpenAPIRouter),
		"/openapi/cluster":      new(multiclusterhandler.OpenAPIRouter),
		"/openapi/logs":         new(loghandler.OpenAPIRouter),
		"/openapi/ticket":       new(tickethandler.OpenAPIRouter),
	} {
		r.Inject(router.Group(name))
	}

	// no auth required
	router.GET("/api/hub/connect", multiclusterhandler.ClusterConnectFromAgent)

	router.GET("/api/kodespace/downloadUrl", commonhandler.GetToolDownloadURL)

	// inject aslan related APIs
	for name, r := range map[string]injector{
		"/api/project":           new(projecthandler.Router),
		"/api/code":              new(codehosthandler.Router),
		"/api/system":            new(systemhandler.Router),
		"/api/service":           new(servicehandler.Router),
		"/api/environment":       new(environmenthandler.Router),
		"/api/cron":              new(cronhandler.Router),
		"/api/workflow":          new(workflowhandler.Router),
		"/api/release_plan":      new(releaseplanhandler.Router),
		"/api/sprint_management": new(sprintmanagementhandler.Router),
		"/api/build":             new(buildhandler.Router),
		"/api/delivery":          new(deliveryhandler.Router),
		"/api/logs":              new(loghandler.Router),
		"/api/testing":           new(testinghandler.Router),
		"/api/quality":           new(testinghandler.QualityCenterRouter),
		"/api/cluster":           new(multiclusterhandler.Router),
		"/api/template":          new(templatehandler.Router),
		"/api/collaboration":     new(collaborationhandler.Router),
		"/api/stat":              new(stathandler.Router),
		"/api/cache":             cachehandler.NewRouter(),
		"/api/vm":                new(vmhandler.Router),
		"/api/ticket":            new(tickethandler.Router),
	} {
		r.Inject(router.Group(name))
	}

	// inject config related APIs
	for _, r := range []injector{
		new(connectorHandler.Router),
		new(emailHandler.Router),
		new(configcodehostHandler.Router),
		new(featuresHandler.Router),
	} {
		r.Inject(router.Group("/api/v1"))
	}

	// inject podexec service API(s)
	podexec := router.Group("/api/podexec")
	{
		podexec.GET("/:productName/:podName/:containerName/podExec/:envName", podexecservice.ServeWs)
		podexec.GET("/production/:productName/:podName/:containerName/podExec/:envName", podexecservice.ServeWs)
		podexec.GET("/debug/:workflowName/:jobName/task/:taskID", podexecservice.DebugWorkflow)
	}

	// inject picket APIs
	for _, r := range []injector{
		new(evaluationhandler.Router),
		new(filterhandler.Router),
	} {
		r.Inject(router.Group("/api/v1/picket"))
	}

	for _, r := range []injector{
		new(publichandler.Router),
	} {
		r.Inject(router.Group("/public-api/v1"))
	}

	router.GET("/api/apidocs/*any", ginswagger.WrapHandler(swaggerfiles.Handler))

	// prometheus metrics API
	handlefunc := func(c *gin.Context) {
		metrics.UpdatePodMetrics()

		runningCustomQueue := workflowcontroller.RunningTasks()
		pendingCustomQueue := workflowcontroller.PendingTasks()

		metrics.SetRunningWorkflows(int64(len(runningCustomQueue)))
		metrics.SetPendingWorkflows(int64(len(pendingCustomQueue)))

		metrics.Cluster.Reset()
		clusterStatusMap := clusterservice.GetClusterStatus()
		for clusterName, status := range clusterStatusMap {
			metrics.SetClusterStatus(clusterName, status)
		}

		promhttp.HandlerFor(metrics.Metrics, promhttp.HandlerOpts{}).ServeHTTP(c.Writer, c.Request)
	}
	router.GET("/api/metrics", handlefunc)
}

type injector interface {
	Inject(router *gin.RouterGroup)
}
