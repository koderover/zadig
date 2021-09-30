/*
Copyright 2021 The KodeRover Authors.

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

package bundle

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/27149chen/afero"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

const (
	manifestPath     = ".manifest"
	policyPath       = "authz.rego"
	rolesPath        = "roles/data.json"
	rolebindingsPath = "bindings/data.json"
)

const (
	MethodView   = "VIEW"
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodPut    = http.MethodPut
	MethodPatch  = http.MethodPatch
	MethodDelete = http.MethodDelete
)

var AllMethods = []string{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete}

var cacheFS afero.Fs

type opaDataSpec struct {
	data interface{}
	path string
}

type opaRoles struct {
	Roles []*role `json:"roles"`
}

type opaRoleBindings struct {
	RoleBindings []*roleBinding `json:"role_bindings"`
}

type opaManifest struct {
	Revision string   `json:"revision"`
	Roots    []string `json:"roots"`
}

type role struct {
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	Rules     []*rule `json:"rules"`
}

type rule struct {
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

type roleBinding struct {
	User     string     `json:"user"`
	RoleRefs []*roleRef `json:"role_refs"`
}

type roleRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type opaData []*opaDataSpec

func (o *opaData) save() error {
	var err error

	cacheFS = afero.NewMemMapFs()
	for _, file := range *o {
		var content []byte
		switch c := file.data.(type) {
		case []byte:
			content = c
		default:
			content, err = json.MarshalIndent(c, "", "  ")
			if err != nil {
				log.Errorf("Failed to marshal file %s, err: %s", file.path, err)
				return err
			}
		}

		err = cacheFS.MkdirAll(filepath.Dir(file.path), 0755)
		if err != nil {
			log.Errorf("Failed to create path %s, err: %s", filepath.Dir(file.path), err)
			return err
		}
		err = afero.WriteFile(cacheFS, file.path, content, 0644)
		if err != nil {
			log.Errorf("Failed to write file %s, err: %s", file.path, err)
			return err
		}
	}

	tarball := "bundle.tar.gz"
	path := filepath.Join(config.DataPath(), tarball)
	if err = fsutil.Tar(afero.NewIOFS(cacheFS), path); err != nil {
		log.Errorf("Failed to archive tarball %s, err: %s", path, err)
		return err
	}

	return nil
}

// TODO: sort the response to get stable data.
func generateOPARoles(roles []*models.Role) *opaRoles {
	data := &opaRoles{}

	for _, ro := range roles {
		opaRole := &role{Name: ro.Name, Namespace: ro.Namespace}
		for _, r := range ro.Rules {
			if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
				r.Methods = AllMethods
			}
			for _, method := range r.Methods {
				for _, endpoint := range r.Endpoints {
					opaRole.Rules = append(opaRole.Rules, &rule{Method: method, Endpoint: endpoint})
				}
			}
		}

		data.Roles = append(data.Roles, opaRole)
	}

	return data
}

// TODO: sort the response to get stable data.
func generateOPARoleBindings(bindings []*models.RoleBinding) *opaRoleBindings {
	data := &opaRoleBindings{}

	userRoleMap := make(map[string][]*roleRef)

	for _, rb := range bindings {
		for _, s := range rb.Subjects {
			if s.Kind == models.UserKind {
				userRoleMap[s.Name] = append(userRoleMap[s.Name], &roleRef{Name: rb.RoleRef.Name, Namespace: rb.RoleRef.Namespace})
			}
		}
	}

	for k, v := range userRoleMap {
		data.RoleBindings = append(data.RoleBindings, &roleBinding{User: k, RoleRefs: v})
	}

	return data
}

func generateOPAManifest() *opaManifest {
	return &opaManifest{
		Revision: "",
		Roots:    []string{""},
	}
}

func generateOPAPolicy() []byte {
	return authz
}

func GenerateOPABundle() error {
	log.Info("Generating OPA bundle")
	defer log.Info("OPA bundle is generated")

	roles, err := mongodb.NewRoleColl().List()
	if err != nil {
		log.Errorf("Failed to list roles, err: %s", err)
	}
	bindings, err := mongodb.NewRoleBindingColl().List()
	if err != nil {
		log.Errorf("Failed to list roleBindings, err: %s", err)
	}

	data := &opaData{
		{data: generateOPAManifest(), path: manifestPath},
		{data: generateOPAPolicy(), path: policyPath},
		{data: generateOPARoles(roles), path: rolesPath},
		{data: generateOPARoleBindings(bindings), path: rolebindingsPath},
	}

	return data.save()
}
