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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/policy/core/yamlconfig"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/opa"
)

const (
	manifestPath   = ".manifest"
	policyRegoPath = "authz.rego"
	rolesPath      = "roles/data.json"
	policiesPath   = "policies/data.json"
	bindingsPath   = "bindings/data.json"

	exemptionsPath = "exemptions/data.json"
	resourcesPath  = "resources/data.json"

	policyRoot       = "rbac"
	rolesRoot        = "roles"
	rolebindingsRoot = "bindings"
	exemptionsRoot   = "exemptions"
	resourcesRoot    = "resources"
	policiesRoot     = "policies"
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

type opaPolicies struct {
	Roles roles `json:"policies"`
}

type opaRoleBindings struct {
	RoleBindings   roleBindings   `json:"role_bindings"`
	PolicyBindings policyBindings `json:"policy_bindings"`
}

type role struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Rules     Rules  `json:"rules"`
}

type Rule struct {
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

func NewFrom(attrs []models.MatchAttribute) Attributes {
	var att Attributes
	for _, v := range attrs {
		a := Attribute{
			Key:   v.Key,
			Value: v.Value,
		}
		att = append(att, &a)
	}
	return att
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

type policyBinding struct {
	UID      string         `json:"uid"`
	Bindings bindingPolicys `json:"bindings"`
}

type binding struct {
	Namespace string   `json:"namespace"`
	RoleRefs  roleRefs `json:"role_refs"`
}

type bindingPolicy struct {
	Namespace string   `json:"namespace"`
	RoleRefs  roleRefs `json:"policy_refs"`
}

type roleRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Rules []*Rule

func (o Rules) Len() int      { return len(o) }
func (o Rules) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o Rules) Less(i, j int) bool {
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

type bindingPolicys []*bindingPolicy

func (o bindingPolicys) Len() int      { return len(o) }
func (o bindingPolicys) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o bindingPolicys) Less(i, j int) bool {
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

type policyBindings []*policyBinding

func (o policyBindings) Len() int      { return len(o) }
func (o policyBindings) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o policyBindings) Less(i, j int) bool {
	return o[i].UID < o[j].UID
}

func generateOPARoles(roles []*models.Role, policyMetas []*models.PolicyMeta) *opaRoles {
	data := &opaRoles{}
	resourceMappings := getResourceActionMappings(false, policyMetas)

	for _, ro := range roles {
		verbAttrMap := make(map[string]sets.String)
		resourceVerbs := make(map[string]sets.String)
		for _, r := range ro.Rules {
			if r.Resources[0] == "ProductionEnvironment" {
				continue
			}
			for _, verb := range r.Verbs {
				if verbs, ok := resourceVerbs[r.Resources[0]]; ok {
					for _, v := range r.Verbs {
						verbs.Insert(v)
					}
					resourceVerbs[r.Resources[0]] = verbs
				} else {
					verbSet := sets.String{}
					for _, v := range r.Verbs {
						verbSet.Insert(v)
					}
					resourceVerbs[r.Resources[0]] = verbSet
				}
				if attrs, ok := verbAttrMap[verb]; ok {
					for _, attribute := range r.MatchAttributes {
						attrs.Insert(attribute.Key + "&&" + attribute.Value)
					}
					verbAttrMap[verb] = attrs
				} else {
					attrSet := sets.String{}
					for _, attribute := range r.MatchAttributes {
						attrSet.Insert(attribute.Key + "&&" + attribute.Value)
					}
					verbAttrMap[verb] = attrSet
				}
			}
		}

		//TODO - mouuii change role model to policy model
		opaRole := &role{Name: ro.Name, Namespace: ro.Namespace}
		for resource, verbs := range resourceVerbs {
			ruleList := resourceMappings.GetPolicyRules(resource, verbs.List(), verbAttrMap)
			opaRole.Rules = append(opaRole.Rules, ruleList...)
		}
		for _, r := range ro.Rules {
			if r.Resources[0] == "ProductionEnvironment" {
				continue
			}
			if r.Kind != models.KindResource {
				if len(r.Verbs) == 1 && r.Verbs[0] == models.MethodAll {
					r.Verbs = AllMethods
				}
				for _, v := range r.Verbs {
					for _, endpoint := range r.Resources {
						opaRole.Rules = append(opaRole.Rules, &Rule{Method: v, Endpoint: endpoint})
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

func generateOPAPolicies(policies []*models.Policy, policyMetas []*models.PolicyMeta) *opaPolicies {
	data := &opaPolicies{}
	resourceMappings := getResourceActionMappings(true, policyMetas)

	for _, policy := range policies {
		verbAttrMap := make(map[string]sets.String)
		resourceVerbs := make(map[string]sets.String)
		for _, r := range policy.Rules {
			for _, verb := range r.Verbs {
				if verbs, ok := resourceVerbs[r.Resources[0]]; ok {
					for _, v := range r.Verbs {
						verbs.Insert(v)
					}
					resourceVerbs[r.Resources[0]] = verbs
				} else {
					verbSet := sets.String{}
					for _, v := range r.Verbs {
						verbSet.Insert(v)
					}
					resourceVerbs[r.Resources[0]] = verbSet
				}
				if attrs, ok := verbAttrMap[verb]; ok {
					for _, attribute := range r.MatchAttributes {
						attrs.Insert(attribute.Key + "&&" + attribute.Value)
					}
					verbAttrMap[verb] = attrs
				} else {
					attrSet := sets.String{}
					for _, attribute := range r.MatchAttributes {
						attrSet.Insert(attribute.Key + "&&" + attribute.Value)
					}
					verbAttrMap[verb] = attrSet
				}
			}
		}

		//TODO - mouuii change role model to policy model
		opaRole := &role{Name: policy.Name, Namespace: policy.Namespace}
		for resource, verbs := range resourceVerbs {
			ruleList := resourceMappings.GetPolicyRules(resource, verbs.List(), verbAttrMap)
			opaRole.Rules = append(opaRole.Rules, ruleList...)
		}
		for _, r := range policy.Rules {
			if r.Kind != models.KindResource {
				if len(r.Verbs) == 1 && r.Verbs[0] == models.MethodAll {
					r.Verbs = AllMethods
				}
				for _, v := range r.Verbs {
					for _, endpoint := range r.Resources {
						opaRole.Rules = append(opaRole.Rules, &Rule{Method: v, Endpoint: endpoint})
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

func generateOPABindings(rbs []*models.RoleBinding, pbs []*models.PolicyBinding) *opaRoleBindings {
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

	userPolicyMap := make(map[string]map[string][]*roleRef)

	for _, rb := range pbs {
		for _, s := range rb.Subjects {
			if s.Kind == models.UserKind {
				if _, ok := userPolicyMap[s.UID]; !ok {
					userPolicyMap[s.UID] = make(map[string][]*roleRef)
				}
				userPolicyMap[s.UID][rb.Namespace] = append(userPolicyMap[s.UID][rb.Namespace], &roleRef{Name: rb.PolicyRef.Name, Namespace: rb.PolicyRef.Namespace})
			}
		}
	}

	for u, nb := range userPolicyMap {
		var bindingsData []*bindingPolicy
		for n, b := range nb {
			sort.Sort(roleRefs(b))
			bindingsData = append(bindingsData, &bindingPolicy{Namespace: n, RoleRefs: b})
		}
		sort.Sort(bindingPolicys(bindingsData))
		data.PolicyBindings = append(data.PolicyBindings, &policyBinding{UID: u, Bindings: bindingsData})
	}

	sort.Sort(data.PolicyBindings)

	return data
}

type ExemptionURLs struct {
	Public     Rules `json:"public"`     // public urls are not controlled by AuthN and AuthZ
	Privileged Rules `json:"privileged"` // privileged urls can only be visited by system admins
	Registered Rules `json:"registered"` // registered urls are the entire list of urls which are controlled by AuthZ, which means that if an url is not in this list, it is not controlled by AuthZ
}

func generateOPAExemptionURLs(policies []*models.PolicyMeta) *ExemptionURLs {
	data := &ExemptionURLs{}

	for _, r := range yamlconfig.GetExemptionsUrls().Public {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			data.Public = append(data.Public, &Rule{Method: method, Endpoint: r.Endpoint})

		}
	}
	sort.Sort(data.Public)

	for _, r := range yamlconfig.GetExemptionsUrls().SystemAdmin {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			data.Privileged = append(data.Privileged, &Rule{Method: method, Endpoint: r.Endpoint})
		}
	}
	sort.Sort(data.Privileged)

	resourceMappings := getResourceActionMappings(false, policies)
	for _, resourceMappings := range resourceMappings {
		for _, rs := range resourceMappings {
			for _, r := range rs {
				data.Registered = append(data.Registered, r)
			}
		}
	}

	adminURLs := append(yamlconfig.GetExemptionsUrls().SystemAdmin, yamlconfig.GetExemptionsUrls().ProjectAdmin...)

	for _, r := range adminURLs {
		if len(r.Methods) == 1 && r.Methods[0] == models.MethodAll {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			data.Registered = append(data.Registered, &Rule{Method: method, Endpoint: r.Endpoint})
		}
	}

	sort.Sort(data.Registered)

	return data
}

func generateOPAPolicyRego() []byte {
	return authz
}

func GenerateOPABundle() error {
	rs, err := mongodb.NewRoleColl().List()
	if err != nil {
		log.Errorf("Failed to list roles, err: %s", err)
	}
	bs, err := mongodb.NewRoleBindingColl().List()
	if err != nil {
		log.Errorf("Failed to list roleBindings, err: %s", err)
	}

	pbs, err := mongodb.NewPolicyBindingColl().List()
	if err != nil {
		log.Errorf("Failed to list roleBindings, err: %s", err)
	}

	pms, err := mongodb.NewPolicyMetaColl().List()
	if err != nil {
		log.Errorf("Failed to list policyMetas, err: %s", err)
	}
	policies, err := mongodb.NewPolicyColl().List()
	if err != nil {
		log.Errorf("Failed to list policies, err: %s", err)
	}

	bundle := &opa.Bundle{
		Data: []*opa.DataSpec{
			{Data: generateOPAPolicyRego(), Path: policyRegoPath},
			{Data: generateOPARoles(rs, pms), Path: rolesPath},
			{Data: generateOPAPolicies(policies, pms), Path: policiesPath},
			{Data: generateOPABindings(bs, pbs), Path: bindingsPath},
			{Data: generateOPAExemptionURLs(pms), Path: exemptionsPath},
			{Data: generateResourceBundle(), Path: resourcesPath},
		},
		Roots: []string{policyRoot, rolesRoot, rolebindingsRoot, exemptionsRoot, resourcesRoot, policiesRoot},
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
