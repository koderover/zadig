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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/tool/log"
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
