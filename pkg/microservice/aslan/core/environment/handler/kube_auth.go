/*
Copyright 2026 The KodeRover Authors.

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

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	clusterservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func newKubeResourceContext(c *gin.Context, permission clusterservice.KubeResourcePermission) *internalhandler.Context {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return ctx
	}
	args := clusterservice.KubeResourceArgs{}
	if err := c.ShouldBindQuery(&args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return ctx
	}
	if clusterservice.CanReadAllKubeResources(ctx.Resources, permission) {
		return ctx
	}
	if permission == clusterservice.KubeClusterInventory {
		ctx.UnAuthorized = true
		return ctx
	}
	if clusterID := c.Param("clusterID"); clusterID != "" {
		args.ClusterID = clusterID
	}
	if args.ClusterID == "" {
		args.ClusterID = setting.LocalClusterID
	}
	relations, err := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{ClusterID: args.ClusterID})
	if err != nil {
		ctx.RespErr = err
		return ctx
	}
	permitted, err := clusterservice.CanSelectKubeResources(ctx, args, permission, relations, internalhandler.ListAuthorizedWorkflowWithVerb)
	ctx.RespErr = err
	ctx.UnAuthorized = !permitted
	return ctx
}
