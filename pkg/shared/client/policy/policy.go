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

package policy

import (
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	policyservice "github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type CreatePoliciesArgs struct {
	Policies []*types.Policy `json:"policies"`
}

type DeletePoliciesArgs struct {
	Names []string `json:"names"`
}

type PolicyBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Policy string               `json:"policy"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

func (c *Client) CreatePolicyBinding(projectName string, policyBindings []*PolicyBinding) error {
	policyBindingList := make([]*policyservice.PolicyBinding, 0)
	for _, binding := range policyBindings {
		policyBindingList = append(policyBindingList, &policyservice.PolicyBinding{
			Name:   binding.Name,
			UID:    binding.UID,
			Policy: binding.Policy,
			Preset: binding.Preset,
			Type:   binding.Type,
		})
	}
	err := policyservice.CreatePolicyBindings(projectName, policyBindingList, log.SugaredLogger())
	return err
}

func (c *Client) CreatePolicies(ns string, request CreatePoliciesArgs) error {
	policyList := make([]*policyservice.Policy, 0)
	for _, policy := range request.Policies {
		ruleList := make([]*policyservice.Rule, 0)
		for _, rule := range policy.Rules {
			attributeList := make([]models.MatchAttribute, 0)
			for _, attr := range rule.MatchAttributes {
				attributeList = append(attributeList, models.MatchAttribute{
					Key:   attr.Key,
					Value: attr.Value,
				})
			}
			ruleList = append(ruleList, &policyservice.Rule{
				Verbs:           rule.Verbs,
				Resources:       rule.Resources,
				Kind:            rule.Kind,
				MatchAttributes: attributeList,
			})
		}
		policyList = append(policyList, &policyservice.Policy{
			Name:        policy.Name,
			Description: policy.Description,
			UpdateTime:  policy.UpdateTime,
			Rules:       ruleList,
		})
	}

	err := policyservice.CreatePolicies(ns, policyList, log.SugaredLogger())

	return err
}

func (c *Client) DeletePolicies(ns string, request DeletePoliciesArgs) error {
	return policyservice.DeletePolicies(request.Names, ns, log.SugaredLogger())
}

func (c *Client) DeletePolicyBindings(names []string, projectName string) error {
	return policyservice.DeletePolicyBindings(names, projectName, "", log.SugaredLogger())
}

func (c *Client) UpdatePolicy(ns string, policy *types.Policy) error {
	ruleList := make([]*policyservice.Rule, 0)
	for _, rule := range policy.Rules {
		attributeList := make([]models.MatchAttribute, 0)
		for _, attr := range rule.MatchAttributes {
			attributeList = append(attributeList, models.MatchAttribute{
				Key:   attr.Key,
				Value: attr.Value,
			})
		}
		ruleList = append(ruleList, &policyservice.Rule{
			Verbs:           rule.Verbs,
			Resources:       rule.Resources,
			Kind:            rule.Kind,
			MatchAttributes: attributeList,
		})
	}

	policyNew := &policyservice.Policy{
		Name:        policy.Name,
		Description: policy.Description,
		UpdateTime:  policy.UpdateTime,
		Rules:       ruleList,
	}

	return policyservice.UpdateOrCreatePolicy(ns, policyNew, log.SugaredLogger())
}

// TODO: Seems like this is a deprecated api
//type ResourcePermissionReq struct {
//	ProjectName  string   `json:"project_name"      form:"project_name"`
//	Uid          string   `json:"uid"               form:"uid"`
//	Resources    []string `json:"resources"         form:"resources"`
//	ResourceType string   `json:"resource_type"     form:"resource_type"`
//}
//
//func (c *Client) GetResourcePermission(req *ResourcePermissionReq) (map[string][]string, error) {
//	url := fmt.Sprintf("/permission/resources")
//	result := make(map[string][]string)
//	_, err := c.Post(url, httpclient.SetResult(req), httpclient.SetResult(&result))
//	if err != nil {
//		log.Errorf("Failed to get resourcePermission,err: %s", err)
//		return nil, err
//	}
//	return result, nil
//}

func (c *Client) GetPolicies(names string) ([]*types.Policy, error) {
	resp, err := policyservice.GetPolicies(names, log.SugaredLogger())
	if err != nil {
		log.Errorf("Failed to getPolicies,err: %s", err)
		return nil, err
	}
	res := make([]*types.Policy, 0)
	for _, policy := range resp {
		ruleList := make([]*types.Rule, 0)
		for _, rule := range policy.Rules {
			attributeList := make([]types.MatchAttribute, 0)
			for _, attr := range rule.MatchAttributes {
				attributeList = append(attributeList, types.MatchAttribute{
					Key:   attr.Key,
					Value: attr.Value,
				})
			}
			ruleList = append(ruleList, &types.Rule{
				Verbs:           rule.Verbs,
				Resources:       rule.Resources,
				Kind:            rule.Kind,
				MatchAttributes: attributeList,
			})
		}
		res = append(res, &types.Policy{
			Name:        policy.Name,
			Description: policy.Description,
			UpdateTime:  policy.UpdateTime,
			Rules:       ruleList,
		})
	}
	return res, nil
}
