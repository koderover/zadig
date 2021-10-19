package policy

import (
	"fmt"

	"github.com/koderover/zadig/pkg/setting"
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

func (c *Client) CreateOrUpdatePolicy(p *Policy) error {
	url := fmt.Sprintf("/policies/%s", p.Resource)

	_, err := c.Put(url, httpclient.SetBody(p))

	return err
}

type RoleBinding struct {
	Name   string
	User   string
	Role   setting.RoleType
	Global bool
}

func (c *Client) CreateRoleBinding(projectName string, roleBinding *RoleBinding) error {
	url := fmt.Sprintf("/rolebindings?projectName=%s", projectName)
	_, err := c.Post(url, httpclient.SetBody(roleBinding))
	return err
}
