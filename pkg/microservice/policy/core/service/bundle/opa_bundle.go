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
	"net/http"
	"sort"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/opa"
)

const (
	manifestPath     = ".manifest"
	policyPath       = "authz.rego"
	rolesPath        = "roles/data.json"
	rolebindingsPath = "bindings/data.json"
	exemptionsPath   = "exemptions/data.json"
	resourcesPath    = "resources/data.json"

	policyRoot       = "rbac"
	rolesRoot        = "roles"
	rolebindingsRoot = "bindings"
	exemptionsRoot   = "exemptions"
	resourcesRoot    = "resources"
)

type expressionOperator string

const (
	OperatorIn           expressionOperator = "In"
	OperatorNotIn        expressionOperator = "NotIn"
	OperatorExists       expressionOperator = "Exists"
	OperatorDoesNotExist expressionOperator = "DoesNotExist"
)

const (
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodPut    = http.MethodPut
	MethodPatch  = http.MethodPatch
	MethodDelete = http.MethodDelete
)

var AllMethods = []string{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete}

var revision string

type opaRoles struct {
	Roles roles `json:"roles"`
}

type opaRoleBindings struct {
	RoleBindings roleBindings `json:"role_bindings"`
}

type role struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Rules     rules  `json:"rules"`
}

type rule struct {
	Method           string       `json:"method"`
	Endpoint         string       `json:"endpoint"`
	ResourceType     string       `json:"resourceType,omitempty"`
	IDRegex          string       `json:"idRegex,omitempty"`
	MatchAttributes  Attributes   `json:"matchAttributes,omitempty"`
	MatchExpressions []expression `json:"matchExpressions,omitempty"`
}

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Attributes []*Attribute

func (a Attributes) LessOrEqual(other Attributes) bool {
	if len(a) == 0 {
		return true
	}
	if len(other) == 0 {
		return false
	}
	for i, attr := range a {
		if i > len(other)-1 {
			return false
		}
		ki := attr.Key
		kj := other[i].Key
		vi := attr.Value
		vj := other[i].Value
		if ki == kj {
			if vi == vj {
				continue
			}
			return vi < vj
		}
		return ki < kj
	}

	return true
}

type expression struct {
	Key      string             `json:"key"`
	Values   []string           `json:"values"`
	Operator expressionOperator `json:"operator"`
}

type roleBinding struct {
	UID      string   `json:"uid"`
	Bindings bindings `json:"bindings"`
}

type binding struct {
	Namespace string   `json:"namespace"`
	RoleRefs  roleRefs `json:"role_refs"`
}

type roleRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type rules []*rule

func (o rules) Len() int      { return len(o) }
func (o rules) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o rules) Less(i, j int) bool {
	if o[i].Endpoint == o[j].Endpoint {
		if o[i].Method == o[j].Method {
			return o[i].MatchAttributes.LessOrEqual(o[j].MatchAttributes)
		}
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

type bindings []*binding

func (o bindings) Len() int      { return len(o) }
func (o bindings) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o bindings) Less(i, j int) bool {
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
	return o[i].UID < o[j].UID
}

func generateOPARoles(roles []*models.Role, policies []*models.Policy) *opaRoles {
	data := &opaRoles{}
	resourceMappings := getResourceActionMappings(policies)

	for _, ro := range roles {
		opaRole := &role{Name: ro.Name, Namespace: ro.Namespace}
		for _, r := range ro.Rules {
			if r.Kind == models.KindResource {
				for _, res := range r.Resources {
					opaRole.Rules = append(opaRole.Rules, resourceMappings.GetRules(res, r.Verbs)...)
				}
			} else {
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

func generateOPARoleBindings(rbs []*models.RoleBinding) *opaRoleBindings {
	data := &opaRoleBindings{}

	userRoleMap := make(map[string]map[string][]*roleRef)

	for _, rb := range rbs {
		for _, s := range rb.Subjects {
			if s.Kind == models.UserKind {
				if _, ok := userRoleMap[s.UID]; !ok {
					userRoleMap[s.UID] = make(map[string][]*roleRef)
				}
				userRoleMap[s.UID][rb.Namespace] = append(userRoleMap[s.UID][rb.Namespace], &roleRef{Name: rb.RoleRef.Name, Namespace: rb.RoleRef.Namespace})
			}
		}
	}

	for u, nb := range userRoleMap {
		var bindingsData []*binding
		for n, b := range nb {
			sort.Sort(roleRefs(b))
			bindingsData = append(bindingsData, &binding{Namespace: n, RoleRefs: b})
		}
		sort.Sort(bindings(bindingsData))
		data.RoleBindings = append(data.RoleBindings, &roleBinding{UID: u, Bindings: bindingsData})
	}

	sort.Sort(data.RoleBindings)

	return data
}

func generateOPAExemptionURLs(policies []*models.Policy) *exemptionURLs {
	data := &exemptionURLs{}

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

	for _, r := range systemAdminURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			for _, endpoint := range r.Endpoints {
				data.Privileged = append(data.Privileged, &rule{Method: method, Endpoint: endpoint})
			}
		}
	}
	sort.Sort(data.Privileged)

	resourceMappings := getResourceActionMappings(policies)
	for _, resourceMappings := range resourceMappings {
		for _, rs := range resourceMappings {
			for _, r := range rs {
				data.Registered = append(data.Registered, r)
			}
		}
	}
	for _, r := range adminURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			for _, endpoint := range r.Endpoints {
				data.Registered = append(data.Registered, &rule{Method: method, Endpoint: endpoint})
			}
		}
	}

	sort.Sort(data.Registered)

	return data
}

func generateOPAPolicy() []byte {
	return authz
}

func GenerateOPABundle() error {
	log.Info("Generating OPA bundle")
	defer log.Info("OPA bundle is generated")

	rs, err := mongodb.NewRoleColl().List()
	if err != nil {
		log.Errorf("Failed to list roles, err: %s", err)
	}
	bs, err := mongodb.NewRoleBindingColl().List()
	if err != nil {
		log.Errorf("Failed to list roleBindings, err: %s", err)
	}
	ps, err := mongodb.NewPolicyColl().List()
	if err != nil {
		log.Errorf("Failed to list policies, err: %s", err)
	}

	bundle := &opa.Bundle{
		Data: []*opa.DataSpec{
			{Data: generateOPAPolicy(), Path: policyPath},
			{Data: generateOPARoles(rs, ps), Path: rolesPath},
			{Data: generateOPARoleBindings(bs), Path: rolebindingsPath},
			{Data: generateOPAExemptionURLs(ps), Path: exemptionsPath},
			{Data: generateResourceBundle(), Path: resourcesPath},
		},
		Roots: []string{policyRoot, rolesRoot, rolebindingsRoot, exemptionsRoot, resourcesRoot},
	}

	hash, err := bundle.Rehash()
	if err != nil {
		log.Errorf("Failed to calculate bundle hash, err: %s", err)
		return err
	}
	revision = hash

	return bundle.Save(config.DataPath())
}

func GetRevision() string {
	return revision
}
