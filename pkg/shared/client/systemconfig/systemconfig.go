package systemconfig

import (
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/service"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Connector struct {
	service.ConnectorBase

	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

func (c *Client) GetConnector(id string) (*service.Connector, error) {
	url := "/connectors/" + id

	res := &service.Connector{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, err
}
