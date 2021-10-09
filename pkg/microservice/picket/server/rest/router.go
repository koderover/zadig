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

	evaluationhandler "github.com/koderover/zadig/pkg/microservice/picket/core/evaluation/handler"
)

func (s *engine) injectRouterGroup(router *gin.RouterGroup) {
	for _, r := range []injector{
		new(evaluationhandler.Router),
	} {
		r.Inject(router.Group("/api/v1"))
	}
}

type injector interface {
	Inject(router *gin.RouterGroup)
}
