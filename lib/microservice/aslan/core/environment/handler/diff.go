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

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
)

//func ConfigDiff(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//	envName := c.Query("envName")
//
//	ctx.Resp, ctx.Err = service.ConfigDiff(envName, c.Param("productName"), c.Param("serviceName"), c.Param("configName"), ctx.Logger)
//	return
//}

func ServiceDiff(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	envName := c.Query("envName")

	ctx.Resp, ctx.Err = service.ServiceDiff(envName, c.Param("productName"), c.Param("serviceName"), ctx.Logger)
	return
}
