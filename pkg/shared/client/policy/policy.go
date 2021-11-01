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
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
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

//func (c *Client) CreateOrUpdateSystemRoleBinding(roleBinding *RoleBinding) error {
//	url := fmt.Sprintf("/rolebindings/system-roles/%s?projectName=%s", roleBinding.Name, projectName)
//	_, err := c.Put(url, httpclient.SetBody(roleBinding))
//	return err
//}

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
