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

package codehost

import (
	"errors"
	"strings"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/shared/poetry"
)

const (
	GitLabProvider  = "gitlab"
	GitHubProvider  = "github"
	GerritProvider  = "gerrit"
	CodeHubProvider = "codehub"
)

type CodeHost struct {
	ID          int    `json:"id"`
	OrgID       int    `json:"orgId"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"accessToken"`
	Namespace   string `json:"namespace"`
}

type Option struct {
	CodeHostType string
	Address      string
	Namespace    string
	CodeHostID   int
}

type Detail struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Owner      string `json:"repoowner"`
	Source     string `json:"source"`
	OauthToken string `json:"oauth_token"`
	Region     string `json:"region"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AccessKey  string `json:"applicationId"`
	SecretKey  string `json:"clientSecret"`
}

func GetCodeHostList() ([]*poetry.CodeHost, error) {
	poetryClient := poetry.New(config.PoetryServiceAddress(), config.PoetryAPIRootKey())
	return poetryClient.ListCodeHosts()
}

func GetCodeHostInfo(option *Option) (*poetry.CodeHost, error) {
	codeHosts, err := GetCodeHostList()
	if err != nil {
		return nil, err
	}

	for _, codeHost := range codeHosts {
		if option.CodeHostID != 0 && codeHost.ID == option.CodeHostID {
			return codeHost, nil
		} else if option.CodeHostID == 0 && option.CodeHostType != "" {
			switch option.CodeHostType {
			case GitLabProvider, GerritProvider, CodeHubProvider:
				if strings.Contains(option.Address, codeHost.Address) {
					return codeHost, nil
				}
			case GitHubProvider:
				if strings.Contains(option.Address, codeHost.Address) && option.Namespace == codeHost.Namespace {
					return codeHost, nil
				}
			}
		}
	}

	return nil, errors.New("not find codeHost")
}

func GetCodeHostInfoByID(id int) (*poetry.CodeHost, error) {
	return GetCodeHostInfo(&Option{CodeHostID: id})
}

func GetCodehostDetail(codehostID int) (*Detail, error) {
	codehost, err := GetCodeHostInfo(&Option{CodeHostID: codehostID})
	if err != nil {
		return nil, err
	}
	detail := &Detail{
		codehostID,
		"",
		codehost.Address,
		codehost.Namespace,
		codehost.Type,
		codehost.AccessToken,
		codehost.Region,
		codehost.Username,
		codehost.Password,
		codehost.AccessKey,
		codehost.SecretKey,
	}

	return detail, nil
}

func ListCodehostDetial() ([]*Detail, error) {
	codehosts, err := GetCodeHostList()
	if err != nil {
		return nil, err
	}

	var details []*Detail

	for _, codehost := range codehosts {
		details = append(details, &Detail{
			codehost.ID,
			"",
			codehost.Address,
			codehost.Namespace,
			codehost.Type,
			codehost.AccessToken,
			codehost.Region,
			codehost.Username,
			codehost.Password,
			codehost.AccessKey,
			codehost.SecretKey,
		})
	}

	return details, nil
}
