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
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/multicluster/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ListClusters(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	clusters, found := internalhandler.GetResourcesInHeader(c)
	if found && len(clusters) == 0 {
		ctx.Resp = []*service.K8SCluster{}
		return
	}

	ctx.Resp, ctx.Err = service.ListClusters(clusters, c.Query("projectName"), ctx.Logger)
}

func GetCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetCluster(c.Param("id"), ctx.Logger)
}

func CreateCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.K8SCluster)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Clean(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	args.CreatedAt = time.Now().Unix()
	args.CreatedBy = ctx.UserName

	ctx.Resp, ctx.Err = service.CreateCluster(args, ctx.Logger)
}

func UpdateCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.K8SCluster)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	if err := args.Clean(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to clean args: %s", err)
		return
	}

	ctx.Resp, ctx.Err = service.UpdateCluster(c.Param("id"), args, ctx.Logger)
}

func DeleteCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DeleteCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

func DisconnectCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DisconnectCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

func ReconnectCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.ReconnectCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

func ClusterConnectFromAgent(c *gin.Context) {
	c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/api/hub")
	service.ProxyAgent(c.Writer, c.Request)
	c.Abort()
}

func GetClusterYaml(hubURI string) func(*gin.Context) {
	return func(c *gin.Context) {
		ctx := internalhandler.NewContext(c)
		defer func() {
			if ctx.Err != nil {
				c.JSON(e.ErrorMessage(ctx.Err))
				c.Abort()
				return
			}
		}()

		yaml, err := service.GetYaml(
			c.Param("id"),
			hubURI,
			strings.HasPrefix(c.Query("type"), "deploy"),
			ctx.Logger,
		)

		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddErr(err)
			return
		}

		c.Data(200, "text/plain", yaml)
		c.Abort()
	}
}

func UpgradeAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.UpgradeAgent(c.Param("id"), ctx.Logger)
}

func CheckEphemeralContainers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.CheckEphemeralContainers(c, c.Param("id"))
}

