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

	codehosthandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/handler"
	connectorHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/connector/handler"
	emailHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/handler"
	featuresHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/handler"
	jiraHandler "github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/handler"
)

func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
	for _, r := range []injector{
		new(connectorHandler.Router),
		new(emailHandler.Router),
		new(jiraHandler.Router),
		new(codehosthandler.Router),
		new(featuresHandler.Router),
	} {
		r.Inject(router.Group("/api/v1"))
	}
}

type injector interface {
	Inject(router *gin.RouterGroup)
}
