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

package service

import (
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

type KubeResourcePermission int

const (
	KubeClusterInventory KubeResourcePermission = iota
	KubeNamespaceCandidates
	KubeWorkflowCandidates
	KubeClusterCandidates
)

type KubeResourceQuery struct {
	ProjectName string `form:"projectName"`
	ClusterID   string `form:"clusterId"`
	Production  *bool  `form:"production"`
}

func CanReadAllKubeResources(resources *user.AuthorizedResources, permission KubeResourcePermission) bool {
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
	return (permission == KubeWorkflowCandidates || permission == KubeClusterCandidates) && actions.Template != nil && (actions.Template.Create || actions.Template.Edit)
}

func CanSelectKubeResources(ctx *internalhandler.Context, query KubeResourceQuery, permission KubeResourcePermission,
	relations []*commonmodels.ProjectClusterRelation,
	listCollaboration func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error)) (bool, error) {
	var collaborationErr error
	var collaboration *user.CollModeAuthorizedWorkflowWithVerb
	var collaborationLoaded bool
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
		if permission == KubeNamespaceCandidates || permission == KubeClusterCandidates {
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
		}
		if permission == KubeWorkflowCandidates || permission == KubeClusterCandidates {
			if auth.Workflow != nil && (auth.Workflow.Create || auth.Workflow.Edit) {
				return true, nil
			}
			if !collaborationLoaded {
				collaboration, collaborationErr = listCollaboration(ctx.UserID, "")
				collaborationLoaded = true
			}
			if collaborationErr != nil {
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
