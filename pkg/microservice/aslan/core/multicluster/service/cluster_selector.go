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
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// ClusterSelectorResp contains only fields needed to select a deployment target.
type ClusterSelectorResp struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
	Status     setting.K8SClusterStatus `json:"status"`
	Production bool                     `json:"production"`
	Type       string                   `json:"type"`
	Local      bool                     `json:"local"`
}

func ListClusterSelectors(ctx *internalhandler.Context, ids []string, args KubeResourceArgs) ([]*ClusterSelectorResp, error) {
	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{IDs: ids})
	if err != nil {
		return nil, err
	}
	var relations []*commonmodels.ProjectClusterRelation
	if args.ProjectName != "" || !CanReadAllKubeResources(ctx.Resources, KubeClusterCandidates) {
		relations, err = commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{})
		if err != nil {
			return nil, err
		}
	}
	return filterClusterSelectors(ctx, args, clusters, relations, internalhandler.ListAuthorizedWorkflowWithVerb)
}

func filterClusterSelectors(ctx *internalhandler.Context, args KubeResourceArgs, clusters []*commonmodels.K8SCluster,
	relations []*commonmodels.ProjectClusterRelation,
	listCollaboration func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error)) ([]*ClusterSelectorResp, error) {
	resp := make([]*ClusterSelectorResp, 0)
	hasGlobalPermission := CanReadAllKubeResources(ctx.Resources, KubeClusterCandidates)
	var collaboration *user.CollModeAuthorizedWorkflowWithVerb
	var collaborationErr error
	var collaborationLoaded bool
	listCollaborationWithCache := func(uid, project string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		if !collaborationLoaded {
			collaboration, collaborationErr = listCollaboration(uid, "")
			collaborationLoaded = true
		}
		return collaboration, collaborationErr
	}
	for _, cluster := range clusters {
		args.ClusterID = cluster.ID.Hex()
		if hasGlobalPermission && args.ProjectName != "" {
			assigned := false
			for _, relation := range relations {
				if relation.ClusterID == args.ClusterID && (relation.ProjectName == args.ProjectName || relation.ProjectName == setting.AllProjects) {
					assigned = true
					break
				}
			}
			if !assigned {
				continue
			}
		}
		if !hasGlobalPermission {
			permitted, err := CanSelectKubeResources(ctx, args, KubeClusterCandidates, relations, listCollaborationWithCache)
			if err != nil {
				return nil, err
			}
			if !permitted {
				continue
			}
		}
		clusterType := cluster.Type
		if clusterType == "" {
			clusterType = setting.AgentClusterType
		}
		resp = append(resp, &ClusterSelectorResp{
			ID:         cluster.ID.Hex(),
			Name:       cluster.Name,
			Status:     cluster.Status,
			Production: cluster.Production,
			Type:       clusterType,
			Local:      cluster.Local,
		})
	}
	return resp, nil
}
