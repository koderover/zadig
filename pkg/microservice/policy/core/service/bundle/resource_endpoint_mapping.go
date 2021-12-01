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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
)

type resourceActionMappings map[string]map[string][]*rule

func (m resourceActionMappings) GetRules(resource string, actions []string) []*rule {
	mappings, ok := m[resource]
	if !ok {
		return nil
	}

	all := false
	if len(actions) == 1 && actions[0] == models.MethodAll {
		all = true
	}
	actionSet := sets.NewString(actions...)
	var res []*rule
	for action, r := range mappings {
		if all || actionSet.Has(action) {
			res = append(res, r...)
		}
	}

	return res
}

func getResourceActionMappings(policies []*models.Policy) resourceActionMappings {
	data := make(resourceActionMappings)
	for _, p := range policies {
		if _, ok := data[p.Resource]; !ok {
			data[p.Resource] = make(map[string][]*rule)
		}

		for _, r := range p.Rules {
			for _, ar := range r.Rules {
				var as []*Attribute
				for _, a := range ar.MatchAttributes {
					as = append(as, &Attribute{Key: a.Key, Value: a.Value})
				}

				data[p.Resource][r.Action] = append(data[p.Resource][r.Action], &rule{
					Method:          ar.Method,
					Endpoint:        ar.Endpoint,
					ResourceType:    ar.ResourceType,
					IDRegex:         ar.IDRegex,
					MatchAttributes: as,
				})
			}
		}
	}

	return data
}
