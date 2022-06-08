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
	"fmt"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
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
	url := fmt.Sprintf("/policybindings?projectName=%s", projectName)
	_, err := c.Post(url, httpclient.SetBody(policyBindings))
	return err
}

func (c *Client) CreatePolicies(ns string, request CreatePoliciesArgs) error {
	url := fmt.Sprintf("/policies?projectName=%s", ns)

	_, err := c.Post(url, httpclient.SetBody(request))
	if err != nil {
		log.Errorf("Failed to createPolicies, error: %s", err)
		return err
	}

	return nil
}

func (c *Client) DeletePolicies(ns string, request DeletePoliciesArgs) error {
	url := fmt.Sprintf("/policies/bulk-delete?projectName=%s", ns)

	_, err := c.Post(url, httpclient.SetBody(request))
	if err != nil {
		log.Errorf("Failed to deletePolicies, error: %s", err)
		return err
	}

	return nil
}

func (c *Client) DeletePolicyBindings(names []string, projectName string) error {
	url := fmt.Sprintf("/policybindings/bulk-delete?projectName=%s", projectName)
	nameArgs := &NameArgs{}
	for _, v := range names {
		nameArgs.Names = append(nameArgs.Names, v)
	}
	_, err := c.Post(url, httpclient.SetBody(nameArgs))
	return err
}

func (c *Client) UpdatePolicy(ns string, policy *types.Policy) error {
	url := fmt.Sprintf("/policies/%s?projectName=%s", policy.Name, ns)
	_, err := c.Put(url, httpclient.SetBody(policy))
	return err
}

type ResourcePermissionReq struct {
	ProjectName  string   `json:"project_name"      form:"project_name"`
	Uid          string   `json:"uid"               form:"uid"`
	Resources    []string `json:"resources"         form:"resources"`
	ResourceType string   `json:"resource_type"     form:"resource_type"`
}

func (c *Client) GetResourcePermission(req *ResourcePermissionReq) (map[string][]string, error) {
	url := fmt.Sprintf("/permission/resources")
	result := make(map[string][]string)
	_, err := c.Post(url, httpclient.SetResult(req), httpclient.SetResult(&result))
	if err != nil {
		log.Errorf("Failed to get resourcePermission,err: %s", err)
		return nil, err
	}
	return result, nil
}

func (c *Client) GetPolicies(names string) ([]*types.Policy, error) {
	url := fmt.Sprintf("/policies/bulk")
	res := make([]*types.Policy, 0)
	_, err := c.Get(url, httpclient.SetQueryParam("names", names), httpclient.SetResult(&res))
	if err != nil {
		log.Errorf("Failed to getPolicies,err: %s", err)
		return nil, err
	}
	return res, nil
}
