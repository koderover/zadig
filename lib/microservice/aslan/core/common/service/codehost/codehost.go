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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/util"
)

const (
	GitLabProvider = "gitlab"
	GitHubProvider = "github"
	GerritProvider = "gerrit"
)

type CodeHost struct {
	ID          int    `json:"id"`
	OrgId       int    `json:"orgId"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"accessToken"`
	Namespace   string `json:"namespace"`
}

type CodeHostOption struct {
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
}

func GetCodehostList(orgID int) ([]*CodeHost, error) {
	uri := fmt.Sprintf("%s/directory/codehostss/search?orgId=1", config.PoetryAPIServer())
	header := http.Header{}
	header.Set("authorization", fmt.Sprintf("X-ROOT-API-KEY %s", config.PoetryAPIRootKey()))
	body, err := util.SendRequest(uri, http.MethodGet, header, nil)
	if err != nil {
		return nil, err
	}

	var codeHosts []*CodeHost
	err = json.Unmarshal(body, &codeHosts)
	if err != nil {
		return nil, err
	}

	return codeHosts, nil
}

func GetCodeHostInfo(option *CodeHostOption) (*CodeHost, error) {
	codeHosts, err := GetCodehostList(1)
	if err != nil {
		return nil, err
	}

	for _, codeHost := range codeHosts {
		if option.CodeHostID != 0 && codeHost.ID == option.CodeHostID {
			return codeHost, nil
		} else if option.CodeHostID == 0 && option.CodeHostType != "" {
			switch option.CodeHostType {
			case GitLabProvider, GerritProvider:
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

func GetCodeHostInfoByID(id int) (*CodeHost, error) {
	return GetCodeHostInfo(&CodeHostOption{CodeHostID: id})
}

func GetCodehostDetail(codehostId int) (*Detail, error) {
	codehost, err := GetCodeHostInfo(&CodeHostOption{CodeHostID: codehostId})
	if err != nil {
		return nil, err
	}
	detail := &Detail{
		codehostId,
		"",
		codehost.Address,
		codehost.Namespace,
		codehost.Type,
		codehost.AccessToken,
	}

	return detail, nil
}

func ListCodehostDetial() ([]*Detail, error) {
	codehosts, err := GetCodehostList(1)
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
		})
	}

	return details, nil
}
