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

package service

import (
	"encoding/json"

	"github.com/dexidp/dex/connector/gitea"
	"github.com/dexidp/dex/connector/github"
	"github.com/dexidp/dex/connector/gitlab"
	"github.com/dexidp/dex/connector/google"
	"github.com/dexidp/dex/connector/ldap"
	"github.com/dexidp/dex/connector/linkedin"
	"github.com/dexidp/dex/connector/microsoft"
	"github.com/dexidp/dex/connector/oidc"
)

type ConnectorType string

const (
	TypeLDAP      ConnectorType = "ldap"
	TypeGitHub    ConnectorType = "github"
	TypeGitlab    ConnectorType = "gitlab"
	TypeOIDC      ConnectorType = "oidc"
	TypeGitea     ConnectorType = "gitea"
	TypeGoogle    ConnectorType = "google"
	TypeLinkedIn  ConnectorType = "linkedin"
	TypeMicrosoft ConnectorType = "microsoft"
)

type Connector struct {
	ConnectorBase

	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

type ConnectorBase struct {
	Type ConnectorType `json:"type"`
}

func (c *Connector) UnmarshalJSON(data []byte) error {
	cb := &ConnectorBase{}
	if err := json.Unmarshal(data, cb); err != nil {
		return err
	}

	switch cb.Type {
	case TypeLDAP:
		c.Config = &ldap.Config{}
	case TypeGitHub:
		c.Config = &github.Config{}
	case TypeGitlab:
		c.Config = &gitlab.Config{}
	case TypeOIDC:
		c.Config = &oidc.Config{}
	case TypeGitea:
		c.Config = &gitea.Config{}
	case TypeGoogle:
		c.Config = &google.Config{}
	case TypeLinkedIn:
		c.Config = &linkedin.Config{}
	case TypeMicrosoft:
		c.Config = &microsoft.Config{}
	}

	type tmp Connector

	return json.Unmarshal(data, (*tmp)(c))
}
