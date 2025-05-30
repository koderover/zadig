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
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func ListClusters(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	clusters, found := internalhandler.GetResourcesInHeader(c)
	if found && len(clusters) == 0 {
		ctx.Resp = []*service.K8SCluster{}
		return
	}

	projectName := c.Query("projectName")
	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "" {
			// TODO: Authorization leak
			//if !ctx.Resources.SystemActions.ClusterManagement.View {
			//	ctx.UnAuthorized = true
			//	return
			//}
		} else {
			if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListClusters(clusters, projectName, ctx.Logger)
}

func GetCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetCluster(c.Param("id"), ctx.Logger)
}

func CreateCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(service.K8SCluster)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Clean(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	args.CreatedAt = time.Now().Unix()
	args.CreatedBy = ctx.UserName
	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateCluster(args, ctx.Logger)
}

func UpdateCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(service.K8SCluster)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	if err := args.Clean(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to clean args: %s", err)
		return
	}

	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateCluster(ctx, c.Param("id"), args)
}

// @Summary 添加/更新集群调度策略
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	id				path		string								true	"集群ID"
// @Param 	body 			body 		service.ScheduleStrategy			true 	"body"
// @Success 200 			{object} 	commonmodels.K8SCluster
// @Router /api/aslan/cluster/clusters/{id}/strategy [put]
func AddOrUpdateClusterStrategy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(service.ScheduleStrategy)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.AddOrUpdateClusterStrategy(ctx, c.Param("id"), args)
}

// @Summary 删除集群调度策略
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	id				path		string								true	"集群ID"
// @Param 	strategyID		query		string								true	"调度策略ID"
// @Success 200 			{object} 	commonmodels.K8SCluster
// @Router /api/aslan/cluster/clusters/{id}/strategy [delete]
func DeleteClusterStrategy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.DeleteClusterStrategy(ctx, c.Param("id"), c.Query("strategyID"))
}

// @Summary 更新集群缓存
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	id				path		string								true	"集群ID"
// @Param 	body 			body 		types.Cache							true 	"body"
// @Success 200 			{object} 	commonmodels.K8SCluster
// @Router /api/aslan/cluster/clusters/{id}/cache [put]
func UpdateClusterCache(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(types.Cache)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateClusterCache(ctx, c.Param("id"), args)
}

// @Summary 更新集群共享存储
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	id				path		string								true	"集群ID"
// @Param 	body 			body 		types.ShareStorage					true 	"body"
// @Success 200 			{object} 	commonmodels.K8SCluster
// @Router /api/aslan/cluster/clusters/{id}/storage [put]
func UpdateClusterStorage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(types.ShareStorage)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateClusterStorage(ctx, c.Param("id"), args)
}

// @Summary 更新集群Dind配置
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	id				path		string								true	"集群ID"
// @Param 	body 			body 		commonmodels.DindCfg				true 	"body"
// @Success 200 			{object} 	commonmodels.K8SCluster
// @Router /api/aslan/cluster/clusters/{id}/dind [put]
func UpdateClusterDind(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.DindCfg)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateClusterDind(ctx, c.Param("id"), args)
}

func GetDeletionInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetClusterDeletionInfo(c.Param("id"), ctx.Logger)
}

func DeleteCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

func GetClusterStrategyReferences(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	clusterID := c.Param("id")
	if strings.TrimSpace(clusterID) == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid clusterID")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetClusterStrategyReferences(clusterID, ctx.Logger)
}

func DisconnectCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DisconnectCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

func ReconnectCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.ReconnectCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}

// @Summary 验证集群连接
// @Description
// @Tags 	cluster
// @Accept 	json
// @Produce json
// @Param 	body 			body 		service.K8SCluster			true 	"body"
// @Success 200
// @Router /api/aslan/cluster/clusters/validate [post]
func ValidateCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit && !ctx.Resources.SystemActions.ClusterManagement.View && !ctx.Resources.SystemActions.ClusterManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(service.K8SCluster)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.RespErr = service.ValidateCluster(args, ctx.Logger)
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
			if ctx.RespErr != nil {
				c.JSON(e.ErrorMessage(ctx.RespErr))
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
			ctx.RespErr = e.ErrInvalidParam.AddErr(err)
			return
		}

		c.Data(200, "text/plain", yaml)
		c.Abort()
	}
}

func UpgradeAgent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.UpgradeAgent(c.Param("id"), ctx.Logger)
}

func CheckEphemeralContainers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.CheckEphemeralContainers(c, c.Query("projectName"), c.Query("envName"))
}

func GetIRSAInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.ClusterManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetClusterIRSAInfo(c.Query("id"), ctx.Logger)
}
