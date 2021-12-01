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

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Policy struct {
	Resource    string  `json:"resource"`
	Alias       string  `json:"alias"`
	Description string  `json:"description"`
	Rules       []*Rule `json:"rules"`
}

type Rule struct {
	Action      string        `json:"action"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*ActionRule `json:"rules"`
}

type ActionRule struct {
	Method          string       `json:"method"`
	Endpoint        string       `json:"endpoint"`
	ResourceType    string       `json:"resourceType,omitempty"`
	IDRegex         string       `json:"idRegex,omitempty"`
	MatchAttributes []*Attribute `json:"matchAttributes,omitempty"`
}

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RoleBinding struct {
	Name   string `json:"name"`
	UID    string `json:"uid"`
	Role   string `json:"role"`
	Public bool   `json:"public"`
}

func (c *Client) CreateOrUpdatePolicy(p *Policy) error {
	url := fmt.Sprintf("/policies/%s", p.Resource)

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

func (c *Client) CreatePublicRole(name string, role *Role) error {
	url := fmt.Sprintf("/public-roles/%s", name)
	_, err := c.Put(url, httpclient.SetBody(role))
	return err
}

type Role struct {
	Name  string `json:"name"`
	Rules []*struct {
		Verbs     []string `json:"verbs"`
		Resources []string `json:"resources"`
		Kind      string   `json:"kind"`
	} `json:"rules"`
}

func (c *Client) Healthz() error {
	url := "/healthz"
	_, err := c.Get(url)
	return err
}
