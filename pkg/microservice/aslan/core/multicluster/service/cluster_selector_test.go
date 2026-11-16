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
	"encoding/json"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestKubeClusterSelectorPermissionsAndFields(t *testing.T) {
	clusters := []*commonmodels.K8SCluster{
		{ID: primitive.ObjectID{1}, Name: "assigned", KubeConfig: "private-kubeconfig", AdvancedConfig: &commonmodels.AdvancedConfig{ClusterAccessYaml: "private-access-yaml"}},
		{ID: primitive.ObjectID{2}, Name: "other-project"},
	}
	relations := []*commonmodels.ProjectClusterRelation{
		{ProjectName: "project", ClusterID: clusters[0].ID.Hex()},
		{ProjectName: "other", ClusterID: clusters[1].ID.Hex()},
	}
	for _, test := range []struct {
		name      string
		resources *user.AuthorizedResources
		project   string
		want      int
	}{
		{name: "template creator without project membership", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{Create: true}}}, want: 2},
		{name: "template editor without cluster view", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{Edit: true}}}, want: 2},
		{name: "template project filter", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{Edit: true}}}, project: "project", want: 1},
		{name: "template viewer denied", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{View: true}}}},
		{name: "project workflow editor", resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {Workflow: &user.WorkflowActions{Edit: true}}}}, want: 1},
		{name: "project environment creator", resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {Env: &user.EnvActions{Create: true}}}}, want: 1},
		{name: "project production service importer", resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {ProductionService: &user.ProductionServiceActions{Create: true}}}}, want: 1},
		{name: "project member denied", resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {}}}},
		{name: "inaccessible project filter", resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}}}, project: "other"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &internalhandler.Context{Resources: test.resources}
			lookup := func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error) { return nil, nil }
			result, err := filterClusterSelectors(ctx, KubeResourceQuery{ProjectName: test.project}, clusters, relations, lookup)
			if err != nil || len(result) != test.want {
				t.Fatalf("got %d clusters, error %v; want %d", len(result), err, test.want)
			}
			for i, item := range result {
				if item.ID != clusters[i].ID.Hex() || item.Name != clusters[i].Name || item.Type != setting.AgentClusterType {
					t.Fatalf("unexpected selector item: %+v", item)
				}
				encoded, err := json.Marshal(item)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]interface{}
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				if len(fields) != 6 {
					t.Fatalf("unexpected selector fields: %s", encoded)
				}
				for _, key := range []string{"id", "name", "status", "production", "type", "local"} {
					if _, ok := fields[key]; !ok {
						t.Fatalf("missing %q: %s", key, encoded)
					}
				}
			}
			if clusters[0].KubeConfig != "private-kubeconfig" || clusters[0].AdvancedConfig.ClusterAccessYaml != "private-access-yaml" {
				t.Fatal("selector projection changed the source cluster")
			}
		})
	}
}

func TestKubeClusterSelectorCollaboration(t *testing.T) {
	ctx := &internalhandler.Context{UserID: "viewer", Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{
		"project": {}, "other": {},
	}}}
	clusters := []*commonmodels.K8SCluster{{ID: primitive.ObjectID{1}}, {ID: primitive.ObjectID{2}}}
	relations := []*commonmodels.ProjectClusterRelation{
		{ProjectName: setting.AllProjects, ClusterID: clusters[0].ID.Hex()},
		{ProjectName: setting.AllProjects, ClusterID: clusters[1].ID.Hex()},
	}
	calls := 0
	lookup := func(uid, project string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		calls++
		if uid != "viewer" || project != "" {
			t.Fatalf("unexpected collaboration scope: %q %q", uid, project)
		}
		return &user.CollModeAuthorizedWorkflowWithVerb{ProjectWorkflowActionsMap: map[string]map[string]*user.WorkflowActions{
			"project": {"workflow": {Edit: true}},
		}}, nil
	}
	result, err := filterClusterSelectors(ctx, KubeResourceQuery{}, clusters, relations, lookup)
	if err != nil || len(result) != 2 || calls != 1 {
		t.Fatalf("got %d clusters, %d lookups, error %v", len(result), calls, err)
	}
	failure := errors.New("authorization unavailable")
	result, err = filterClusterSelectors(ctx, KubeResourceQuery{}, clusters, relations, func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		return nil, failure
	})
	if result != nil || !errors.Is(err, failure) {
		t.Fatalf("authorization failure must not return candidates: %v, %v", result, err)
	}
}
