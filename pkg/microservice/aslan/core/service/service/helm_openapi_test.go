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
	"strings"
	"testing"
)

func TestOpenAPIQueryHelmValuesReqValidateNormalizesPaths(t *testing.T) {
	req := &OpenAPIQueryHelmValuesReq{
		CodehostName: "github",
		Owner:        "owner",
		Repo:         "repo",
		Branch:       "main",
		Paths: []*OpenAPIHelmValuesScanPath{
			{Path: "./values.yaml"},
			{Path: "charts//", IsDir: true},
			{Path: "./", IsDir: true},
		},
	}

	if err := req.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	paths := []string{req.Paths[0].Path, req.Paths[1].Path, req.Paths[2].Path}
	want := []string{"values.yaml", "charts", ""}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("normalized path %d = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestOpenAPIBulkCreateHelmServiceReqValidateNormalizesPaths(t *testing.T) {
	req := &OpenAPIBulkCreateHelmServiceReq{
		TemplateName: "template",
		CodehostName: "github",
		Owner:        "owner",
		Repo:         "repo",
		Branch:       "main",
		ValuesPaths:  []string{"./service.yaml", "charts//worker.yml"},
	}

	if err := req.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if got, want := strings.Join(req.ValuesPaths, ","), "service.yaml,charts/worker.yml"; got != want {
		t.Errorf("normalized values paths = %q, want %q", got, want)
	}
}

func TestOpenAPIHelmValuesPathValidation(t *testing.T) {
	for _, repoPath := range []string{"../values.yaml", "/values.yaml"} {
		req := &OpenAPIQueryHelmValuesReq{
			CodehostName: "github",
			Owner:        "owner",
			Repo:         "repo",
			Branch:       "main",
			Paths:        []*OpenAPIHelmValuesScanPath{{Path: repoPath}},
		}
		if err := req.Validate(); err == nil {
			t.Errorf("Validate() error = nil for path %q", repoPath)
		}
	}
}

func TestOpenAPIHelmValuesPathLimits(t *testing.T) {
	queryReq := &OpenAPIQueryHelmValuesReq{
		CodehostName: "github",
		Owner:        "owner",
		Repo:         "repo",
		Branch:       "main",
		Paths:        make([]*OpenAPIHelmValuesScanPath, maxOpenAPIHelmValuesPaths+1),
	}
	for i := range queryReq.Paths {
		queryReq.Paths[i] = &OpenAPIHelmValuesScanPath{Path: "dir"}
	}
	if err := queryReq.Validate(); err == nil {
		t.Error("QueryHelmValues request with too many paths should fail")
	}

	bulkReq := &OpenAPIBulkCreateHelmServiceReq{
		TemplateName: "template",
		CodehostName: "github",
		Owner:        "owner",
		Repo:         "repo",
		Branch:       "main",
		ValuesPaths:  make([]string, maxOpenAPIHelmValuesPaths+1),
	}
	for i := range bulkReq.ValuesPaths {
		bulkReq.ValuesPaths[i] = "service.yaml"
	}
	if err := bulkReq.Validate(); err == nil {
		t.Error("Bulk create request with too many values paths should fail")
	}
}

func TestOpenAPIHelmValuesContentValid(t *testing.T) {
	if !openAPIHelmValuesContentValid([]byte("replicaCount: 1\n")) {
		t.Error("ordinary values YAML should be valid")
	}
	if openAPIHelmValuesContentValid([]byte("apiVersion: v1\nkind: ConfigMap\n")) {
		t.Error("Kubernetes manifest should not be treated as values YAML")
	}
}
