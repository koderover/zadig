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
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type kubeResourcePermission int

const (
	kubeClusterInventory kubeResourcePermission = iota
	kubeNamespaceCandidates
	kubeWorkflowCandidates
)

type kubeResourceQuery struct {
	ProjectName string `form:"projectName"`
	ClusterID   string `form:"clusterId"`
	Production  *bool  `form:"production"`
}

func newKubeResourceContext(c *gin.Context, permission kubeResourcePermission) *internalhandler.Context {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return ctx
	}
	query := kubeResourceQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return ctx
	}
	if canReadAllKubeResources(ctx.Resources, permission) {
		return ctx
	}
	if permission == kubeClusterInventory {
		ctx.UnAuthorized = true
		return ctx
	}
	if clusterID := c.Param("clusterID"); clusterID != "" {
		query.ClusterID = clusterID
	}
	if query.ClusterID == "" {
		query.ClusterID = setting.LocalClusterID
	}
	relations, err := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{ClusterID: query.ClusterID})
	if err != nil {
		ctx.RespErr = err
		return ctx
	}
	permitted, err := canSelectKubeResources(ctx, query, permission, relations, internalhandler.ListAuthorizedWorkflowWithVerb)
	ctx.RespErr = err
	ctx.UnAuthorized = !permitted
	return ctx
}

func canReadAllKubeResources(resources *user.AuthorizedResources, permission kubeResourcePermission) bool {
	if resources.IsSystemAdmin {
		return true
	}
	actions := resources.SystemActions
	if actions == nil {
		return false
	}
	if actions.ClusterManagement != nil && actions.ClusterManagement.View {
		return true
	}
	// Workflow templates are system-scoped and can select targets across projects.
	return permission == kubeWorkflowCandidates && actions.Template != nil && (actions.Template.Create || actions.Template.Edit)
}

func canSelectKubeResources(ctx *internalhandler.Context, query kubeResourceQuery, permission kubeResourcePermission,
	relations []*commonmodels.ProjectClusterRelation,
	listCollaboration func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error)) (bool, error) {
	var collaborationErr error
	for project, auth := range ctx.Resources.ProjectAuthInfo {
		if auth == nil || (query.ProjectName != "" && query.ProjectName != project) {
			continue
		}
		allowedCluster := false
		for _, relation := range relations {
			if relation.ClusterID == query.ClusterID && (relation.ProjectName == project || relation.ProjectName == setting.AllProjects) {
				allowedCluster = true
				break
			}
		}
		if !allowedCluster {
			continue
		}
		// A project cluster grant covers selector metadata, including unbound namespaces.
		if auth.IsProjectAdmin {
			return true, nil
		}
		switch permission {
		case kubeNamespaceCandidates:
			if query.Production == nil || !*query.Production {
				if (auth.Env != nil && auth.Env.Create) || (auth.Service != nil && auth.Service.Create) {
					return true, nil
				}
			}
			if query.Production == nil || *query.Production {
				if (auth.ProductionEnv != nil && auth.ProductionEnv.Create) || (auth.ProductionService != nil && auth.ProductionService.Create) {
					return true, nil
				}
			}
		case kubeWorkflowCandidates:
			if auth.Workflow != nil && (auth.Workflow.Create || auth.Workflow.Edit) {
				return true, nil
			}
			collaboration, err := listCollaboration(ctx.UserID, project)
			if err != nil {
				collaborationErr = err
				continue
			}
			if collaboration != nil {
				for _, workflow := range collaboration.ProjectWorkflowActionsMap[project] {
					if workflow != nil && workflow.Edit {
						return true, nil
					}
				}
			}
		}
	}
	return false, collaborationErr
}
