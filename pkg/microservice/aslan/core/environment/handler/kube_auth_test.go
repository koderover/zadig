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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestKubeResourceHandlersRejectUnauthenticatedRequests(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	router := gin.New()
	(&Router{}).Inject(router.Group("/environment"))
	for _, path := range []string{
		"available_namespaces?clusterId=other",
		"nodes?clusterId=other",
		"available_namespaces",
		"nodes",
		"namespace/cluster/other",
		"deployment/cluster/other/namespace/private",
		"workload/cluster/other/namespace/private",
		"custom_workload/cluster/other/namespace/private",
		"canary_service/cluster/other/namespace/private",
		"resources/cluster/other/namespace/private",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/environment/kube/"+path, nil))
			if response.Code != http.StatusForbidden {
				t.Fatalf("got HTTP %d, want 403", response.Code)
			}
		})
	}
}

func TestKubeNamespaceAuthorization(t *testing.T) {
	for _, test := range []struct {
		name          string
		project       string
		cluster       string
		namespace     string
		production    bool
		projectAdmin  bool
		devView       bool
		prodView      bool
		collaboration bool
		collabErr     error
		want          bool
		wantCollab    bool
	}{
		{name: "development viewer", devView: true, want: true},
		{name: "production viewer", production: true, prodView: true, want: true},
		{name: "project admin", production: true, projectAdmin: true, want: true},
		{name: "other project in same cluster", project: "other", devView: true},
		{name: "same namespace in other cluster", cluster: "other", devView: true},
		{name: "other namespace in same cluster", namespace: "other", devView: true},
		{name: "development view cannot read production", production: true, devView: true, wantCollab: true},
		{name: "production view cannot read development", prodView: true, wantCollab: true},
		{name: "no view permission", wantCollab: true},
		{name: "development collaboration", collaboration: true, want: true, wantCollab: true},
		{name: "production collaboration", production: true, collaboration: true, want: true, wantCollab: true},
		{name: "collaboration error fails closed", collaboration: true, collabErr: errors.New("unavailable"), wantCollab: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := &commonmodels.Product{ProductName: "project", EnvName: "env", ClusterID: "cluster", Namespace: "namespace", Production: test.production}
			if test.project != "" {
				env.ProductName = test.project
			}
			if test.cluster != "" {
				env.ClusterID = test.cluster
			}
			if test.namespace != "" {
				env.Namespace = test.namespace
			}
			ctx := &internalhandler.Context{UserID: "viewer", Resources: &user.AuthorizedResources{
				ProjectAuthInfo: map[string]*user.ProjectActions{"project": {
					IsProjectAdmin: test.projectAdmin,
					Env:            &user.EnvActions{View: test.devView},
					ProductionEnv:  &user.ProductionEnvActions{View: test.prodView},
				}},
			}}
			called := false
			checkCollaboration := func(uid, project, resource, name, action string) (bool, error) {
				called = true
				wantAction := types.EnvActionView
				if test.production {
					wantAction = types.ProductionEnvActionView
				}
				if uid != "viewer" || project != "project" || resource != types.ResourceTypeEnvironment || name != "env" || action != wantAction {
					t.Fatalf("unexpected collaboration scope: %q %q %q %q %q", uid, project, resource, name, action)
				}
				return test.collaboration, test.collabErr
			}
			if got := canViewKubeNamespace(ctx, "cluster", "namespace", []*commonmodels.Product{env}, checkCollaboration); got != test.want {
				t.Fatalf("allowed = %v, want %v", got, test.want)
			}
			if called != test.wantCollab {
				t.Fatalf("collaboration checked = %v, want %v", called, test.wantCollab)
			}
		})
	}
}

func TestKubeNamespaceUnboundAndLegacyLocalCluster(t *testing.T) {
	ctx := &internalhandler.Context{Resources: &user.AuthorizedResources{
		ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}},
	}}
	checkCollaboration := func(string, string, string, string, string) (bool, error) {
		t.Fatal("unexpected collaboration check")
		return false, nil
	}
	if canViewKubeNamespace(ctx, "cluster", "namespace", nil, checkCollaboration) {
		t.Fatal("project admin must not read an unbound namespace")
	}
	env := &commonmodels.Product{ProductName: "project", Namespace: "namespace"}
	if !canViewKubeNamespace(ctx, setting.LocalClusterID, "namespace", []*commonmodels.Product{env}, checkCollaboration) {
		t.Fatal("legacy empty cluster ID should refer to the local cluster")
	}
	if canViewKubeNamespace(ctx, "remote", "namespace", []*commonmodels.Product{env}, checkCollaboration) {
		t.Fatal("legacy local environment must not authorize a remote cluster")
	}
}
