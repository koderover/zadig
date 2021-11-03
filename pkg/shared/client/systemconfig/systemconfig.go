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

func (c *Client) GetConnector(id string) (*Connector, error) {
	url := "/connectors/" + id

	res := &Connector{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, err
}
