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
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
)

type resourceActionMappings map[string]map[string][]*Rule

func (m resourceActionMappings) GetRules(resource string, actions []string) []*Rule {
	mappings, ok := m[resource]
	if !ok {
		return nil
	}

	all := false
	if len(actions) == 1 && actions[0] == models.MethodAll {
		all = true
	}
	actionSet := sets.NewString(actions...)
	var res []*Rule
	for action, r := range mappings {
		if all || actionSet.Has(action) {
			for _, rr := range r {
				rrr := &Rule{
					Method:           rr.Method,
					Endpoint:         rr.Endpoint,
					ResourceType:     rr.ResourceType,
					IDRegex:          rr.IDRegex,
					MatchAttributes:  rr.MatchAttributes,
					MatchExpressions: rr.MatchExpressions,
				}
				res = append(res, rrr)
			}
		}
	}

	return res
}

func (m resourceActionMappings) GetPolicyRules(resource string, actions []string, verbAttrMap map[string]sets.String) []*Rule {
	mappings, ok := m[resource]
	if !ok {
		return nil
	}

	all := false
	if len(actions) == 1 && actions[0] == models.MethodAll {
		all = true
	}
	actionSet := sets.NewString(actions...)
	var res []*Rule
	for action, r := range mappings {
		var attr Attributes
		if attrs, ok := verbAttrMap[action]; ok {
			for _, s := range attrs.List() {
				attrString := strings.Split(s, "&&")
				attr = append(attr, &Attribute{
					Key:   attrString[0],
					Value: attrString[1],
				})
			}
		}
		if all || actionSet.Has(action) {
			for _, rr := range r {
				attrSets := sets.String{}
				rrAttr := attr
				for _, attribute := range rrAttr {
					attrSets.Insert(attribute.Key + "&&" + attribute.Value)
				}
				for _, attribute := range rr.MatchAttributes {
					if !attrSets.Has(attribute.Key + "&&" + attribute.Value) {
						rrAttr = append(rrAttr, attribute)
						attrSets.Insert(attribute.Key + "&&" + attribute.Value)
					}
				}
				rrr := &Rule{
					Method:           rr.Method,
					Endpoint:         rr.Endpoint,
					ResourceType:     rr.ResourceType,
					IDRegex:          rr.IDRegex,
					MatchAttributes:  rrAttr,
					MatchExpressions: rr.MatchExpressions,
				}
				res = append(res, rrr)
			}
		}
	}

	return res
}

func getResourceActionMappings(isPolicy bool, policies []*models.PolicyMeta) resourceActionMappings {
	data := make(resourceActionMappings)
	for _, p := range policies {
		if _, ok := data[p.Resource]; !ok {
			data[p.Resource] = make(map[string][]*Rule)
		}

		for _, r := range p.Rules {
			for _, ar := range r.Rules {
				var as []*Attribute
				for _, a := range ar.MatchAttributes {
					if isPolicy {
						continue
					}
					as = append(as, &Attribute{Key: a.Key, Value: a.Value})
				}

				data[p.Resource][r.Action] = append(data[p.Resource][r.Action], &Rule{
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
