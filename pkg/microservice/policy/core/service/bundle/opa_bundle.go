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
	"github.com/koderover/zadig/pkg/microservice/policy/core/yamlconfig"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/opa"
	"github.com/koderover/zadig/pkg/types"
)

const (
	manifestPath   = ".manifest"
	policyRegoPath = "authz.rego"

	exemptionsPath = "exemptions/data.json"

	policyRoot       = "rbac"
	rolesRoot        = "roles"
	rolebindingsRoot = "bindings"
	exemptionsRoot   = "exemptions"
	resourcesRoot    = "resources"
	policiesRoot     = "policies"
)

type expressionOperator string

const (
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodPut    = http.MethodPut
	MethodPatch  = http.MethodPatch
	MethodDelete = http.MethodDelete
)

var AllMethods = []string{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete}

var revision string

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

type ExemptionURLs struct {
	Public     Rules `json:"public"`     // public urls are not controlled by AuthN and AuthZ
	Privileged Rules `json:"privileged"` // privileged urls can only be visited by system admins
	Registered Rules `json:"registered"` // registered urls are the entire list of urls which are controlled by AuthZ, which means that if an url is not in this list, it is not controlled by AuthZ
}

func generateOPAExemptionURLs(policies []*types.PolicyMeta) *ExemptionURLs {
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
	policieMetas := yamlconfig.DefaultPolicyMetasConfig().Policies()

	bundle := &opa.Bundle{
		Data: []*opa.DataSpec{
			{Data: generateOPAPolicyRego(), Path: policyRegoPath},
			{Data: generateOPAExemptionURLs(policieMetas), Path: exemptionsPath},
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
