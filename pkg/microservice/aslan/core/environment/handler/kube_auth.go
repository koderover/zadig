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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/types"
)

func newKubeResourceContext(c *gin.Context) *internalhandler.Context {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return ctx
	}
	if ctx.Resources.IsSystemAdmin || (ctx.Resources.SystemActions != nil &&
		ctx.Resources.SystemActions.ClusterManagement != nil && ctx.Resources.SystemActions.ClusterManagement.View) {
		return ctx
	}

	namespace := c.Param("namespace")
	// Cluster-wide inventory and unbound namespaces require cluster-level permission.
	if namespace == "" {
		ctx.UnAuthorized = true
		return ctx
	}
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Namespace: namespace})
	if err != nil {
		ctx.RespErr = err
		return ctx
	}
	ctx.UnAuthorized = !canViewKubeNamespace(ctx, c.Param("clusterID"), namespace, envs, internalhandler.GetCollaborationModePermission)
	return ctx
}

func canViewKubeNamespace(ctx *internalhandler.Context, clusterID, namespace string, envs []*commonmodels.Product,
	checkCollaboration func(string, string, string, string, string) (bool, error)) bool {
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	for _, env := range envs {
		envClusterID := env.ClusterID
		if envClusterID == "" {
			envClusterID = setting.LocalClusterID
		}
		if envClusterID != clusterID || env.Namespace != namespace {
			continue
		}
		projectAuth := ctx.Resources.ProjectAuthInfo[env.ProductName]
		if projectAuth == nil {
			continue
		}
		if projectAuth.IsProjectAdmin {
			return true
		}
		action := types.EnvActionView
		if env.Production {
			action = types.ProductionEnvActionView
			if projectAuth.ProductionEnv != nil && projectAuth.ProductionEnv.View {
				return true
			}
		} else if projectAuth.Env != nil && projectAuth.Env.View {
			return true
		}
		permitted, err := checkCollaboration(ctx.UserID, env.ProductName, types.ResourceTypeEnvironment, env.EnvName, action)
		if err == nil && permitted {
			return true
		}
	}
	return false
}
