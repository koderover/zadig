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
	"errors"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestKubeSelectorBusinessPermissions(t *testing.T) {
	dev, prod := false, true
	for _, test := range []struct {
		name              string
		permission        KubeResourcePermission
		auth              *user.ProjectActions
		production        *bool
		collaboration     *user.WorkflowActions
		collabErr         error
		want              bool
		wantCollaboration bool
	}{
		{
			name:       "create development environment",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{Env: &user.EnvActions{Create: true}},
			want:       true,
		},
		{
			name:       "create production environment",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{ProductionEnv: &user.ProductionEnvActions{Create: true}},
			production: &prod,
			want:       true,
		},
		{
			name:       "import development service",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{Service: &user.ServiceActions{Create: true}},
			want:       true,
		},
		{
			name:       "import production service",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{ProductionService: &user.ProductionServiceActions{Create: true}},
			production: &prod,
			want:       true,
		},
		{
			name:       "legacy caller without production flag",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{ProductionEnv: &user.ProductionEnvActions{Create: true}},
			want:       true,
		},
		{
			name:       "dev creation cannot satisfy explicit production",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{Env: &user.EnvActions{Create: true}},
			production: &prod,
		},
		{
			name:       "prod creation cannot satisfy explicit development",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{ProductionEnv: &user.ProductionEnvActions{Create: true}},
			production: &dev,
		},
		{
			name:       "environment view is insufficient for creation",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{Env: &user.EnvActions{View: true}},
		},
		{name: "project member without permissions", permission: KubeNamespaceCandidates, auth: &user.ProjectActions{}},
		{
			name:       "workflow permission cannot create environment",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{Workflow: &user.WorkflowActions{Edit: true}},
		},
		{
			name:       "project admin creates environment",
			permission: KubeNamespaceCandidates,
			auth:       &user.ProjectActions{IsProjectAdmin: true},
			want:       true,
		},
		{
			name:       "project admin selects workflow targets",
			permission: KubeWorkflowCandidates,
			auth:       &user.ProjectActions{IsProjectAdmin: true},
			want:       true,
		},
		{
			name:       "workflow create without environment",
			permission: KubeWorkflowCandidates,
			auth:       &user.ProjectActions{Workflow: &user.WorkflowActions{Create: true}},
			want:       true,
		},
		{
			name:       "workflow edit without environment",
			permission: KubeWorkflowCandidates,
			auth:       &user.ProjectActions{Workflow: &user.WorkflowActions{Edit: true}},
			want:       true,
		},
		{
			name:              "workflow view is insufficient",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{Workflow: &user.WorkflowActions{View: true}},
			wantCollaboration: true,
		},
		{
			name:              "environment view cannot edit workflow",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{Env: &user.EnvActions{View: true}},
			wantCollaboration: true,
		},
		{
			name:              "service create cannot edit workflow",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{Service: &user.ServiceActions{Create: true}},
			wantCollaboration: true,
		},
		{
			name:              "workflow collaboration edit",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{},
			collaboration:     &user.WorkflowActions{Edit: true},
			want:              true,
			wantCollaboration: true,
		},
		{
			name:              "workflow collaboration view denied",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{},
			collaboration:     &user.WorkflowActions{View: true},
			wantCollaboration: true,
		},
		{
			name:              "workflow collaboration execute denied",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{},
			collaboration:     &user.WorkflowActions{Execute: true},
			wantCollaboration: true,
		},
		{
			name:              "collaboration failure denies access",
			permission:        KubeWorkflowCandidates,
			auth:              &user.ProjectActions{},
			collaboration:     &user.WorkflowActions{Edit: true},
			collabErr:         errors.New("unavailable"),
			wantCollaboration: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &internalhandler.Context{UserID: "viewer", Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": test.auth}}}
			called := false
			listCollaboration := func(uid, project string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
				called = true
				if uid != "viewer" || project != "" {
					t.Fatalf("unexpected collaboration scope: %q %q", uid, project)
				}
				return &user.CollModeAuthorizedWorkflowWithVerb{ProjectWorkflowActionsMap: map[string]map[string]*user.WorkflowActions{"project": {"workflow": test.collaboration}}}, test.collabErr
			}
			relations := []*commonmodels.ProjectClusterRelation{{ProjectName: "project", ClusterID: "cluster"}}
			got, err := CanSelectKubeResources(ctx, KubeResourceArgs{ClusterID: "cluster", Production: test.production}, test.permission, relations, listCollaboration)
			if got != test.want || (err != nil) != (test.collabErr != nil) {
				t.Fatalf("allowed = %v, error = %v, want %v", got, err, test.want)
			}
			if called != test.wantCollaboration {
				t.Fatalf("collaboration checked = %v, want %v", called, test.wantCollaboration)
			}
		})
	}
}

func TestKubeSelectorClusterScope(t *testing.T) {
	for _, test := range []struct {
		name            string
		projectFilter   string
		relationProject string
		relationCluster string
		want            bool
	}{
		{name: "project cluster", relationProject: "project", relationCluster: "cluster", want: true},
		{name: "all projects cluster", relationProject: setting.AllProjects, relationCluster: "cluster", want: true},
		{name: "other project cluster", relationProject: "other", relationCluster: "cluster"},
		{name: "other cluster", relationProject: "project", relationCluster: "other"},
		{
			name:            "explicit matching project",
			projectFilter:   "project",
			relationProject: "project",
			relationCluster: "cluster",
			want:            true,
		},
		{
			name:            "explicit inaccessible project",
			projectFilter:   "other",
			relationProject: setting.AllProjects,
			relationCluster: "cluster",
		},
		{name: "unassigned cluster"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &internalhandler.Context{Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}}}}
			listCollaboration := func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
				t.Fatal("unexpected collaboration lookup")
				return nil, nil
			}
			relations := []*commonmodels.ProjectClusterRelation{{ProjectName: test.relationProject, ClusterID: test.relationCluster}}
			for _, permission := range []KubeResourcePermission{KubeNamespaceCandidates, KubeWorkflowCandidates} {
				got, err := CanSelectKubeResources(ctx, KubeResourceArgs{ClusterID: "cluster", ProjectName: test.projectFilter}, permission, relations, listCollaboration)
				if got != test.want || err != nil {
					t.Fatalf("permission %d: allowed = %v, error = %v, want %v", permission, got, err, test.want)
				}
			}
		})
	}
}

func TestKubeGlobalPermissions(t *testing.T) {
	for _, test := range []struct {
		name       string
		resources  *user.AuthorizedResources
		inventory  bool
		namespaces bool
		workflow   bool
	}{
		{
			name:       "system admin",
			resources:  &user.AuthorizedResources{IsSystemAdmin: true},
			inventory:  true,
			namespaces: true,
			workflow:   true,
		},
		{
			name:      "cluster viewer",
			resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{ClusterManagement: &user.ClusterManagementActions{View: true}}},
			inventory: true,
		},
		{
			name:      "template creator",
			resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{Create: true}}},
			workflow:  true,
		},
		{
			name:      "template editor",
			resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{Edit: true}}},
			workflow:  true,
		},
		{
			name:      "cluster viewer with template edit",
			resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{ClusterManagement: &user.ClusterManagementActions{View: true}, Template: &user.TemplateActions{Edit: true}}},
			inventory: true,
			workflow:  true,
		},
		{
			name:      "template viewer",
			resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{Template: &user.TemplateActions{View: true}}},
		},
		{name: "missing system actions", resources: &user.AuthorizedResources{}},
		{
			name:      "project admin has no global grant",
			resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for permission, want := range map[KubeResourcePermission]bool{KubeClusterInventory: test.inventory, KubeNamespaceCandidates: test.namespaces, KubeWorkflowCandidates: test.workflow} {
				if got := CanReadAllKubeResources(test.resources, permission); got != want {
					t.Fatalf("permission %d: allowed = %v, want %v", permission, got, want)
				}
			}
		})
	}
}

func TestKubeWorkflowCollaborationScope(t *testing.T) {
	ctx := &internalhandler.Context{Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{
		"project": {},
	}}}
	relations := []*commonmodels.ProjectClusterRelation{{ProjectName: setting.AllProjects, ClusterID: "cluster"}}
	args := KubeResourceArgs{ClusterID: "cluster"}
	otherProject := func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		return &user.CollModeAuthorizedWorkflowWithVerb{ProjectWorkflowActionsMap: map[string]map[string]*user.WorkflowActions{
			"other": {"workflow": {Edit: true}},
		}}, nil
	}
	if permitted, err := CanSelectKubeResources(ctx, args, KubeWorkflowCandidates, relations, otherProject); permitted || err != nil {
		t.Fatalf("another project's collaboration must not authorize access: %v, %v", permitted, err)
	}
	ctx.Resources.ProjectAuthInfo["direct"] = &user.ProjectActions{Workflow: &user.WorkflowActions{Edit: true}}
	failingCollaboration := func(string, string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
		return nil, errors.New("unavailable")
	}
	for i := 0; i < 20; i++ {
		if permitted, err := CanSelectKubeResources(ctx, args, KubeWorkflowCandidates, relations, failingCollaboration); !permitted || err != nil {
			t.Fatalf("an explicit grant must survive another project's collaboration failure: %v, %v", permitted, err)
		}
	}
}
