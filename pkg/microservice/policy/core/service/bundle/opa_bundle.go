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
	"sort"

	"github.com/27149chen/afero"
	"github.com/google/uuid"

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
	exemptionPath    = "exemptions/data.json"
)

const (
	MethodView             = "VIEW"
	MethodGet              = http.MethodGet
	MethodPost             = http.MethodPost
	MethodPut              = http.MethodPut
	MethodPatch            = http.MethodPatch
	MethodDelete           = http.MethodDelete
	ActionCreate           = "create"
	ActionDelete           = "delete"
	ActionDeleteCollection = "deletecollection"
	ActionGet              = "get"
	ActionList             = "list"
	ActionPatch            = "patch"
	ActionUpdate           = "update"
	ActionWatch            = "watch"
)

var AllMethods = []string{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete}
var AllActions = []string{ActionCreate, ActionDelete, ActionDeleteCollection, ActionGet, ActionList, ActionPatch, ActionUpdate, ActionWatch}

var cacheFS afero.Fs

type opaRoles struct {
	Roles roles `json:"roles"`
}

type opaRoleBindings struct {
	RoleBindings roleBindings `json:"role_bindings"`
}

type opaManifest struct {
	Revision string   `json:"revision"`
	Roots    []string `json:"roots"`
}

type role struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Rules     rules  `json:"rules"`
}

type rule struct {
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

type roleBinding struct {
	User     string   `json:"user"`
	RoleRefs roleRefs `json:"role_refs"`
}

type roleRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type opaDataSpec struct {
	data interface{}
	path string
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
			content, err = json.MarshalIndent(c, "", "    ")
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

type rules []*rule

func (o rules) Len() int      { return len(o) }
func (o rules) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o rules) Less(i, j int) bool {
	if o[i].Endpoint == o[j].Endpoint {
		return o[i].Method < o[j].Method
	}
	return o[i].Endpoint < o[j].Endpoint
}

type roles []*role

func (o roles) Len() int      { return len(o) }
func (o roles) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o roles) Less(i, j int) bool {
	if o[i].Namespace == o[j].Namespace {
		return o[i].Name < o[j].Name
	}
	return o[i].Namespace < o[j].Namespace
}

type roleRefs []*roleRef

func (o roleRefs) Len() int      { return len(o) }
func (o roleRefs) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o roleRefs) Less(i, j int) bool {
	if o[i].Namespace == o[j].Namespace {
		return o[i].Name < o[j].Name
	}
	return o[i].Namespace < o[j].Namespace
}

type roleBindings []*roleBinding

func (o roleBindings) Len() int      { return len(o) }
func (o roleBindings) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o roleBindings) Less(i, j int) bool {
	return o[i].User < o[j].User
}

func generateOPAResourceRoles(roles []*models.Role) *opaRoles {
	data := &opaRoles{}

	for _, ro := range roles {
		opaRole := &role{Name: ro.Name, Namespace: ro.Namespace}
		for _, r := range ro.Rules {
			if len(r.Verbs) == 1 && r.Verbs[0] == models.MethodAll {
				r.Verbs = AllActions
			}
			for _, res := range r.Resources {
				if mapping, ok := mappings[res]; !ok {
					continue
				} else {
					for _, v := range r.Verbs {
						opaRole.Rules = append(opaRole.Rules, mapping[v]...)
					}
				}

			}
		}

		sort.Sort(opaRole.Rules)
		data.Roles = append(data.Roles, opaRole)
	}

	sort.Sort(data.Roles)

	return data
}

func generateOPARoles(roles []*models.Role) *opaRoles {
	data := &opaRoles{}

	for _, ro := range roles {
		opaRole := &role{Name: ro.Name, Namespace: ro.Namespace}
		if ro.Kind == models.KindResource {
			for _, r := range ro.Rules {
				if len(r.Verbs) == 1 && r.Verbs[0] == models.MethodAll {
					r.Verbs = AllActions
				}
				for _, res := range r.Resources {
					if mapping, ok := mappings[res]; !ok {
						continue
					} else {
						for _, v := range r.Verbs {
							opaRole.Rules = append(opaRole.Rules, mapping[v]...)
						}
					}

				}
			}
		} else {
			for _, r := range ro.Rules {
				if len(r.Verbs) == 1 && r.Verbs[0] == models.MethodAll {
					r.Verbs = AllMethods
				}
				for _, v := range r.Verbs {
					for _, endpoint := range r.Resources {
						opaRole.Rules = append(opaRole.Rules, &rule{Method: v, Endpoint: endpoint})
					}
				}
			}
		}

		sort.Sort(opaRole.Rules)
		data.Roles = append(data.Roles, opaRole)
	}

	sort.Sort(data.Roles)

	return data
}

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
		sort.Sort(roleRefs(v))
		data.RoleBindings = append(data.RoleBindings, &roleBinding{User: k, RoleRefs: v})
	}

	sort.Sort(data.RoleBindings)

	return data
}

func generateOPAExemptionURLs() *exemptionURLs {
	data := &exemptionURLs{}

	for _, r := range globalURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			for _, endpoint := range r.Endpoints {
				data.Global = append(data.Global, &rule{Method: method, Endpoint: endpoint})
			}
		}
	}
	sort.Sort(data.Global)

	for _, r := range namespacedURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			for _, endpoint := range r.Endpoints {
				data.Namespaced = append(data.Namespaced, &rule{Method: method, Endpoint: endpoint})
			}
		}
	}

	sort.Sort(data.Namespaced)

	for _, r := range publicURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			for _, endpoint := range r.Endpoints {
				data.Public = append(data.Public, &rule{Method: method, Endpoint: endpoint})
			}
		}
	}

	sort.Sort(data.Public)

	return data
}

func generateOPAManifest() *opaManifest {
	return &opaManifest{
		Revision: uuid.New().String(),
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
		{data: generateOPAExemptionURLs(), path: exemptionPath},
	}

	return data.save()
}

func GetRevision() string {
	data, err := afero.ReadFile(cacheFS, manifestPath)
	if err != nil {
		log.Errorf("Failed to read manifest, err: %s", err)
		return ""
	}

	mf := &opaManifest{}
	if err = json.Unmarshal(data, mf); err != nil {
		log.Errorf("Failed to Unmarshal manifest, err: %s", err)
		return ""
	}

	return mf.Revision
}
