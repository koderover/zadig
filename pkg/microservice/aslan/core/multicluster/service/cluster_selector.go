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

// ClusterSelector contains only fields needed to select a deployment target.
type ClusterSelector struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
	Status     setting.K8SClusterStatus `json:"status"`
	Production bool                     `json:"production"`
	Type       string                   `json:"type"`
	Local      bool                     `json:"local"`
}

func ListClusterSelectors(ctx *internalhandler.Context, ids []string, query KubeResourceQuery) ([]*ClusterSelector, error) {
	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{IDs: ids})
	if err != nil {
		return nil, err
	}
	var relations []*commonmodels.ProjectClusterRelation
	if query.ProjectName != "" || !CanReadAllKubeResources(ctx.Resources, KubeClusterCandidates) {
		relations, err = commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{})
		if err != nil {
			return nil, err
		}
	}
	return filterClusterSelectors(ctx, query, clusters, relations, internalhandler.ListAuthorizedWorkflowWithVerb)
}

func filterClusterSelectors(ctx *internalhandler.Context, query KubeResourceQuery, clusters []*commonmodels.K8SCluster,
	relations []*commonmodels.ProjectClusterRelation,
	listCollaboration func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error)) ([]*ClusterSelector, error) {
	result := make([]*ClusterSelector, 0)
	global := CanReadAllKubeResources(ctx.Resources, KubeClusterCandidates)
	var collaboration *user.CollModeAuthorizedWorkflowWithVerb
	var collaborationErr error
	var loaded bool
	lookup := func(uid, project string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		if !loaded {
			collaboration, collaborationErr = listCollaboration(uid, "")
			loaded = true
		}
		return collaboration, collaborationErr
	}
	for _, cluster := range clusters {
		query.ClusterID = cluster.ID.Hex()
		if global && query.ProjectName != "" {
			assigned := false
			for _, relation := range relations {
				if relation.ClusterID == query.ClusterID && (relation.ProjectName == query.ProjectName || relation.ProjectName == setting.AllProjects) {
					assigned = true
					break
				}
			}
			if !assigned {
				continue
			}
		}
		if !global {
			allowed, err := CanSelectKubeResources(ctx, query, KubeClusterCandidates, relations, lookup)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
		}
		clusterType := cluster.Type
		if clusterType == "" {
			clusterType = setting.AgentClusterType
		}
		result = append(result, &ClusterSelector{
			ID: cluster.ID.Hex(), Name: cluster.Name, Status: cluster.Status,
			Production: cluster.Production, Type: clusterType, Local: cluster.Local,
		})
	}
	return result, nil
}
