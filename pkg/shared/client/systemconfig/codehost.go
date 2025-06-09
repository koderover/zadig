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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	codehostservice "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/service"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

const (
	GitLabProvider  = "gitlab"
	GitHubProvider  = "github"
	GerritProvider  = "gerrit"
	GiteeProvider   = "gitee"
	GiteeEEProvider = "gitee-enterprise"
	OtherProvider   = "other"
)

type CodeHost struct {
	ID           int    `json:"id"`
	Address      string `json:"address"`
	Type         string `json:"type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Namespace    string `json:"namespace"`
	Region       string `json:"region"`
	// the field and tag not consistent because of db field
	AccessKey string `json:"application_id"`
	SecretKey string `json:"client_secret"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	// the field determine whether the proxy is enabled
	EnableProxy        bool           `json:"enable_proxy"`
	UpdatedAt          int64          `json:"updated_at"`
	Alias              string         `json:"alias,omitempty"`
	AuthType           types.AuthType `json:"auth_type,omitempty"`
	SSHKey             string         `json:"ssh_key,omitempty"`
	PrivateAccessToken string         `json:"private_access_token,omitempty"`
	DisableSSL         bool           `json:"disable_ssl"`
	// perforce Type parameters
	P4Host string `json:"perforce_host"`
	P4Port int    `json:"perforce_port"`
}

type Option struct {
	CodeHostType string
	Address      string
	Namespace    string
}

func GetCodeHostInfo(opt *Option) (*CodeHost, error) {
	return New().GetCodeHostByAddressAndOwner(opt.Address, opt.Namespace, opt.CodeHostType)
}

func (c *Client) GetCodeHost(id int) (*CodeHost, error) {
	resp, err := codehostservice.GetCodeHost(id, false, log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	res := &CodeHost{
		ID:                 resp.ID,
		Address:            resp.Address,
		Type:               resp.Type,
		AccessToken:        resp.AccessToken,
		RefreshToken:       resp.RefreshToken,
		Namespace:          resp.Namespace,
		Region:             resp.Region,
		AccessKey:          resp.ApplicationId,
		SecretKey:          resp.ClientSecret,
		Username:           resp.Username,
		Password:           resp.Password,
		DisableSSL:         resp.DisableSSL,
		EnableProxy:        resp.EnableProxy,
		UpdatedAt:          resp.UpdatedAt,
		Alias:              resp.Alias,
		AuthType:           resp.AuthType,
		SSHKey:             resp.SSHKey,
		PrivateAccessToken: resp.PrivateAccessToken,
		P4Host:             resp.P4Host,
		P4Port:             resp.P4Port,
	}

	return res, nil
}

func (c *Client) GetRawCodeHost(id int) (*CodeHost, error) {
	resp, err := codehostservice.GetCodeHost(id, true, log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	res := &CodeHost{
		ID:                 resp.ID,
		Address:            resp.Address,
		Type:               resp.Type,
		AccessToken:        resp.AccessToken,
		RefreshToken:       resp.RefreshToken,
		Namespace:          resp.Namespace,
		Region:             resp.Region,
		AccessKey:          resp.ApplicationId,
		SecretKey:          resp.ClientSecret,
		Username:           resp.Username,
		Password:           resp.Password,
		EnableProxy:        resp.EnableProxy,
		UpdatedAt:          resp.UpdatedAt,
		Alias:              resp.Alias,
		AuthType:           resp.AuthType,
		SSHKey:             resp.SSHKey,
		PrivateAccessToken: resp.PrivateAccessToken,
	}

	return res, nil
}

func (c *Client) ListCodeHostsInternal() ([]*CodeHost, error) {
	resp, err := codehostservice.ListInternal("", "", "", log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	res := make([]*CodeHost, 0)
	for _, ch := range resp {
		res = append(res, &CodeHost{
			ID:                 ch.ID,
			Address:            ch.Address,
			Type:               ch.Type,
			AccessToken:        ch.AccessToken,
			RefreshToken:       ch.RefreshToken,
			Namespace:          ch.Namespace,
			Region:             ch.Region,
			AccessKey:          ch.ApplicationId,
			SecretKey:          ch.ClientSecret,
			Username:           ch.Username,
			Password:           ch.Password,
			EnableProxy:        ch.EnableProxy,
			UpdatedAt:          ch.UpdatedAt,
			Alias:              ch.Alias,
			AuthType:           ch.AuthType,
			SSHKey:             ch.SSHKey,
			PrivateAccessToken: ch.PrivateAccessToken,
		})
	}

	return res, nil
}

func (c *Client) UpdateCodeHost(id int, codehost *CodeHost) error {
	arg := &models.CodeHost{
		ID:                 codehost.ID,
		Type:               codehost.Type,
		Address:            codehost.Address,
		AccessToken:        codehost.AccessToken,
		RefreshToken:       codehost.RefreshToken,
		Namespace:          codehost.Namespace,
		ApplicationId:      codehost.AccessKey,
		Region:             codehost.Region,
		Username:           codehost.Username,
		Password:           codehost.Password,
		ClientSecret:       codehost.SecretKey,
		Alias:              codehost.Alias,
		AuthType:           codehost.AuthType,
		SSHKey:             codehost.SSHKey,
		PrivateAccessToken: codehost.PrivateAccessToken,
		EnableProxy:        codehost.EnableProxy,
		UpdatedAt:          codehost.UpdatedAt,
		DisableSSL:         codehost.DisableSSL,
	}

	_, err := codehostservice.UpdateCodeHost(arg, log.SugaredLogger())
	return err
}

func (c *Client) GetCodeHostByAddressAndOwner(address, owner, source string) (*CodeHost, error) {
	resp, err := codehostservice.ListInternal(address, owner, source, log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no codehost found")
	} else if len(resp) > 1 {
		return nil, fmt.Errorf("more than one codehosts found")
	}

	res := &CodeHost{
		ID:                 resp[0].ID,
		Address:            resp[0].Address,
		Type:               resp[0].Type,
		AccessToken:        resp[0].AccessToken,
		RefreshToken:       resp[0].RefreshToken,
		Namespace:          resp[0].Namespace,
		Region:             resp[0].Region,
		AccessKey:          resp[0].ApplicationId,
		SecretKey:          resp[0].ClientSecret,
		Username:           resp[0].Username,
		Password:           resp[0].Password,
		EnableProxy:        resp[0].EnableProxy,
		UpdatedAt:          resp[0].UpdatedAt,
		Alias:              resp[0].Alias,
		AuthType:           resp[0].AuthType,
		SSHKey:             resp[0].SSHKey,
		PrivateAccessToken: resp[0].PrivateAccessToken,
	}
	return res, nil
}
