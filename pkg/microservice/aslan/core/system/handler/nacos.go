/*
Copyright 2023 The KodeRover Authors.

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

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func ListNacosNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = commonservice.ListNacosNamespace(c.Param("nacosID"), ctx.Logger)
}

// @summary 获取nacos配置列表
// @description
// @tags 	system
// @accept 	json
// @produce json
// @Param   nacosID 			path 		string 							         true 	"nacosID"
// @Param   nacosNamespaceID 	path 		string 							         true 	"nacosNamespaceID"
// @Param   groupName 			query 		string 							         false 	"group name"
// @success 200              	{array}     types.NacosConfig
// @router /api/aslan/system/nacos/{nacosID}/namespace/{nacosNamespaceID} [get]
func ListNacosConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = commonservice.ListNacosConfig(c.Param("nacosID"), c.Param("nacosNamespaceID"), c.Query("groupName"), ctx.Logger)
}

// @summary 获取nacos组列表
// @description
// @tags 	system
// @accept 	json
// @produce json
// @Param   nacosID 			path 		string 							         true 	"nacosID"
// @Param   nacosNamespaceID 	path 		string 							         true 	"nacosNamespaceID"
// @Param   keyword 			query 		string 							         false 	"group name keyword"
// @success 200              	{array}     types.NacosDataID
// @router /api/aslan/system/nacos/{nacosID}/namespace/{nacosNamespaceID}/group [get]
func ListNacosGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = commonservice.ListNacosGroup(c.Param("nacosID"), c.Param("nacosNamespaceID"), c.Query("keyword"), ctx.Logger)
}
