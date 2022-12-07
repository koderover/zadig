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

package systemconfig

import (
	connectorservice "github.com/koderover/zadig/pkg/microservice/systemconfig/core/connector/service"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Connector struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

func (c *Client) GetLDAPConnector(id string) (*Connector, error) {
	resp, err := connectorservice.GetConnector(id, log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	res := &Connector{
		Type:   string(resp.Type),
		ID:     resp.ID,
		Name:   resp.Name,
		Config: resp.Config,
	}

	return res, nil
}

func (c *Client) ListConnectorsInternal() ([]*Connector, error) {
	res := make([]*Connector, 0)

	resp, err := connectorservice.ListConnectorsInternal(log.SugaredLogger())
	for _, connector := range resp {
		res = append(res, &Connector{
			Type:   string(connector.Type),
			ID:     connector.ID,
			Name:   connector.Name,
			Config: connector.Config,
		})
	}

	return res, err
}
