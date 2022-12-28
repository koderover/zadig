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

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	policyservice "github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type RoleBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Role   string               `json:"role"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

func (c *Client) ListRoleBindings(projectName string) ([]*RoleBinding, error) {
	res := make([]*RoleBinding, 0)

	resp, err := policyservice.ListRoleBindings(projectName, "", log.SugaredLogger())
	for _, rb := range resp {
		res = append(res, &RoleBinding{
			Name:   rb.Name,
			UID:    rb.UID,
			Role:   rb.Role,
			Preset: rb.Preset,
			Type:   rb.Type,
		})
	}

	return res, err
}

func (c *Client) CreateOrUpdateRoleBinding(projectName string, roleBinding *RoleBinding) error {
	args := &policyservice.RoleBinding{
		Name:   roleBinding.Name,
		UID:    roleBinding.UID,
		Role:   roleBinding.Role,
		Preset: roleBinding.Preset,
		Type:   roleBinding.Type,
	}
	return policyservice.UpdateOrCreateRoleBinding(projectName, args, log.SugaredLogger())
}

func (c *Client) CreateOrUpdateSystemRoleBinding(roleBinding *RoleBinding) error {
	url := fmt.Sprintf("/system-rolebindings/%s", roleBinding.Name)
	_, err := c.Put(url, httpclient.SetBody(roleBinding))
	return err
}

func (c *Client) DeleteRoleBinding(name string, projectName string) error {
	url := fmt.Sprintf("/rolebindings/%s?projectName=%s", name, projectName)
	_, err := c.Delete(url)
	return err
}

type NameArgs struct {
	Names []string `json:"names"`
}

func (c *Client) DeleteRoleBindings(names []string, projectName string) error {
	return policyservice.DeleteRoleBindings(names, projectName, "", log.SugaredLogger())
}

func (c *Client) DeleteRoles(names []string, projectName string) error {
	return policyservice.DeleteRoles(names, projectName, log.SugaredLogger())
}

// TODO: possibly deprecated since this is not used
//func (c *Client) CreateSystemRole(name string, role *Role) error {
//	url := fmt.Sprintf("/system-roles/%s", name)
//	_, err := c.Put(url, httpclient.SetBody(role))
//	return err
//}

func (c *Client) CreatePresetRole(name string, role *Role) error {
	ruleList := make([]*policyservice.Rule, 0)
	for _, rule := range role.Rules {
		attributeList := make([]models.MatchAttribute, 0)
		for _, attribute := range rule.MatchAttributes {
			attributeList = append(attributeList, models.MatchAttribute{
				Key:   attribute.Key,
				Value: attribute.Value,
			})
		}
		ruleList = append(ruleList, &policyservice.Rule{
			Verbs:           rule.Verbs,
			Resources:       rule.Resources,
			Kind:            rule.Kind,
			MatchAttributes: attributeList,
		})
	}
	arg := &policyservice.Role{
		Name:  role.Name,
		Rules: ruleList,
		Type:  role.Type,
		Desc:  role.Desc,
	}
	return policyservice.CreateRole(policyservice.PresetScope, arg, log.SugaredLogger())
}

type Role struct {
	Name  string               `json:"name"`
	Type  setting.ResourceType `json:"type"`
	Desc  string               `json:"desc"`
	Rules []*struct {
		Verbs           []string               `json:"verbs"`
		Resources       []string               `json:"resources"`
		Kind            string                 `json:"kind"`
		MatchAttributes []types.MatchAttribute `json:"match_attributes,omitempty"`
	} `json:"rules"`
}

func (c *Client) Healthz() error {
	url := "/healthz"
	_, err := c.Get(url)
	return err
}
