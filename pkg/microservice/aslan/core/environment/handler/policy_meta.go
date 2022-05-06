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

package handler

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	productionPolicyResource = "ProductionEnvironment"
	productionPolicyAlias    = "环境(生产/预发布)"
	productionKey            = "production"
	productionValueTrue      = "true"
)

//go:embed policy.yaml
var policyMeta []byte

func (*Router) Policies() []*policy.PolicyMeta {
	res := &policy.PolicyMeta{}
	err := yaml.Unmarshal(policyMeta, res)
	if err != nil {
		// should not have happened here
		log.DPanic(err)
	}
	for i, meta := range res.Rules {
		tmpRules := []*policy.ActionRule{}
		for _, rule := range meta.Rules {
			if rule.ResourceType == "" {
				rule.ResourceType = res.Resource
			}
			needIdregex := strings.Contains(rule.Endpoint, ":name")
			if needIdregex {
				idRegex := strings.ReplaceAll(rule.Endpoint, ":name", `([\w\W].*)`)
				idRegex = strings.ReplaceAll(idRegex, "?*", `[\w\W].*`)
				endpoint := strings.ReplaceAll(rule.Endpoint, ":name", "?*")
				rule.Endpoint = endpoint
				rule.IDRegex = idRegex
				rule.MatchAttributes = []*policy.Attribute{
					{
						Key:   "production",
						Value: "false",
					},
				}
				log.Infof("rule:%+v", rule)
			}

			tmpRules = append(tmpRules, rule)
		}
		res.Rules[i].Rules = tmpRules
	}

	productionPolicy := &policy.PolicyMeta{}
	_ = yaml.Unmarshal(policyMeta, productionPolicy)
	productionPolicy.Resource = productionPolicyResource
	productionPolicy.Alias = productionPolicyAlias
	//productionPolicy.Description = ""
	for _, ru := range productionPolicy.Rules {
		for _, r := range ru.Rules {
			for _, a := range r.MatchAttributes {
				if a.Key == productionKey {
					a.Value = productionValueTrue
				}
			}
		}
	}

	return []*policy.PolicyMeta{res, productionPolicy}
}
