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

func ExportYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	serviceName := c.Query("serviceName")
	envName := c.Query("envName")
	productName := c.Query("productName")

	ctx.Resp = service.ExportYaml(envName, productName, serviceName, ctx.Logger)
	return
}

//func ExportBuildYaml(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//
//	pipeName := c.Param("name")
//
//	if len(pipeName) == 0 {
//		c.JSON(e.ErrorMessage(e.ErrInvalidParam.AddDesc("empty pipeline name")))
//		c.Abort()
//		return
//	}
//
//	resp, err := service.ExportBuildYaml(pipeName, ctx.Logger)
//	if err != nil {
//		c.JSON(e.ErrorMessage(e.ErrInvalidParam.AddDesc("empty file")))
//		c.Abort()
//		return
//	}
//
//	c.Writer.Header().Add("Content-Disposition", "attachment; filename=\"build.yaml\"")
//
//	c.YAML(200, resp)
//	return
//}
