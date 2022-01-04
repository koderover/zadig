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
	"encoding/json"

	"github.com/dexidp/dex/connector/ldap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Connector struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

func (c *Client) GetLDAPConnector(id string) (*Connector, error) {
	url := "/connectors/" + id

	res := &Connector{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	configData, err := json.Marshal(res.Config)
	if err != nil {
		return nil, err
	}

	ldapConfig := &ldap.Config{}
	if err = json.Unmarshal(configData, ldapConfig); err != nil {
		return nil, err
	}

	res.Config = ldapConfig

	return res, err
}

func (c *Client) ListConnectors() ([]*Connector, error) {
	url := "/connectors"

	res := make([]*Connector, 0)
	_, err := c.Get(url, httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, err
}
