/*
Copyright 2023 The KodeRover Authors.

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

package permission

import (
	_ "embed"
	"net/http"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/opa"
	"github.com/koderover/zadig/pkg/types"
)

const (
	policyRegoPath = "authz.rego"
	exemptionsPath = "exemptions/data.json"

	exemptionsRoot = "exemptions"
)

//go:embed rego/urls.yaml
var urls []byte

//go:embed rego/authz.rego
var authz []byte

// hash log to prevent re-creating bundle every time.
var BundleRevision string

func init() {
	if err := yaml.Unmarshal(urls, &defaultURLConfig); err != nil {
		panic(err)
	}
}

func GenerateOPABundle() error {
	bundle := &opa.Bundle{
		Data: []*opa.DataSpec{
			{Data: authz, Path: policyRegoPath},
			{Data: generateOPAExemptionURLs(), Path: exemptionsPath},
		},
		Roots: []string{exemptionsRoot},
	}

	hash, err := bundle.Rehash()
	if err != nil {
		log.Errorf("Failed to calculate bundle hash, err: %s", err)
		return err
	}
	BundleRevision = hash

	return bundle.Save(config.DataPath())
}

type Rule struct {
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

const (
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodPut    = http.MethodPut
	MethodPatch  = http.MethodPatch
	MethodDelete = http.MethodDelete
)

var defaultURLConfig *ConfigYaml
var AllMethods = []string{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete}

type ConfigYaml struct {
	ExemptionUrls *ExemptionURLs `json:"exemption_urls"`
	Description   string         `json:"description"`
}

type ExemptionURLs struct {
	Public []*types.PolicyRule `json:"public"`
}

func generateOPAExemptionURLs() []*Rule {
	resp := make([]*Rule, 0)

	for _, r := range defaultURLConfig.ExemptionUrls.Public {
		if len(r.Methods) == 1 && r.Methods[0] == "*" {
			r.Methods = AllMethods
		}
		for _, method := range r.Methods {
			resp = append(resp, &Rule{Method: method, Endpoint: r.Endpoint})
		}
	}

	return resp
}
