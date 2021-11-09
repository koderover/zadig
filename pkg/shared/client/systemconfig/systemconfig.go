package systemconfig

import (
	"github.com/dexidp/dex/connector/ldap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Connector struct {
	Type   string       `json:"type"`
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Config *ldap.Config `json:"config"`
}

type Email struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	UserName string `json:"username"`
	Password string `json:"password"`
}

func (c *Client) GetLDAPConnector(id string) (*Connector, error) {
	url := "/connectors/" + id

	res := &Connector{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, err
}

func (c *Client) GetEmailHost() (*Email, error) {
	url := "/emails/internal/host/"

	res := &Email{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, err
}
