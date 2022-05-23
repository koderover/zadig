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
	"github.com/koderover/zadig/pkg/types"
)

type RoleBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Role   string               `json:"role"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

func (c *Client) CreateOrUpdatePolicyRegistration(p *types.PolicyMeta) error {
	url := fmt.Sprintf("/policymetas/%s", p.Resource)

	_, err := c.Put(url, httpclient.SetBody(p))

	return err
}

func (c *Client) ListRoleBindings(projectName string) ([]*RoleBinding, error) {
	url := fmt.Sprintf("/rolebindings?projectName=%s", projectName)

	res := make([]*RoleBinding, 0)
	_, err := c.Get(url, httpclient.SetResult(&res))

	return res, err
}

func (c *Client) CreateOrUpdateRoleBinding(projectName string, roleBinding *RoleBinding) error {
	url := fmt.Sprintf("/rolebindings/%s?projectName=%s", roleBinding.Name, projectName)
	_, err := c.Put(url, httpclient.SetBody(roleBinding))
	return err
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
	url := fmt.Sprintf("/rolebindings/bulk-delete?projectName=%s", projectName)
	nameArgs := &NameArgs{}
	for _, v := range names {
		nameArgs.Names = append(nameArgs.Names, v)
	}
	_, err := c.Post(url, httpclient.SetBody(nameArgs))
	return err
}

func (c *Client) DeleteRoles(names []string, projectName string) error {
	url := fmt.Sprintf("/roles/bulk-delete?projectName=%s", projectName)
	nameArgs := &NameArgs{}
	for _, v := range names {
		nameArgs.Names = append(nameArgs.Names, v)
	}
	_, err := c.Post(url, httpclient.SetBody(nameArgs))
	return err
}

func (c *Client) CreateSystemRole(name string, role *Role) error {
	url := fmt.Sprintf("/system-roles/%s", name)
	_, err := c.Put(url, httpclient.SetBody(role))
	return err
}

func (c *Client) CreatePresetRole(name string, role *Role) error {
	url := fmt.Sprintf("/preset-roles/%s", name)
	_, err := c.Put(url, httpclient.SetBody(role))
	return err
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
