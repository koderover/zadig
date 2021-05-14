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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	SystemType       = 1
	ProjectType      = 2
	AllUsersRoleName = "all-users"
)

type UserInfo struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Password       string     `json:"password"`
	Phone          string     `json:"phone"`
	IsAdmin        bool       `json:"isAdmin"`
	IsSuperUser    bool       `json:"isSuperUser"`
	IsTeamLeader   bool       `json:"isTeamLeader"`
	OrganizationID int        `json:"organization_id"`
	Directory      string     `json:"directory"`
	LastLoginAt    int64      `json:"lastLogin"`
	CreatedAt      int64      `json:"created_at"`
	UpdatedAt      int64      `json:"updated_at"`
	Teams          []TeamInfo `json:"teams"`
}

type TeamInfo struct {
	ID           int    `json:"id"`
	OrgID        int    `json:"orgId"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	IsTeamLeader bool   `json:"isTeamLeader"`
	//Users        []*User `json:"leaders"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type Organization struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Website   string `json:"website"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type UserViewReponseModel struct {
	User         *UserInfo     `json:"info"`
	Teams        []*TeamInfo   `json:"teams"`
	Organization *Organization `json:"organization"`
}

type PoetryClient struct {
	PoetryAPIServer string
	ApiRootKey      string
}

func (p *PoetryClient) SendRequest(url, method string, data interface{}, header http.Header) (string, error) {
	if body, err := p.Do(url, method, GetRequestBody(data), header); err != nil {
		return "", err
	} else {
		return string(body), nil
	}
}

func GetRequestBody(body interface{}) io.Reader {
	if body == nil {
		return nil
	}
	dataStr, ok := body.(string)
	if ok {
		return bytes.NewReader([]byte(dataStr))
	}
	dataBytes, ok := body.([]byte)
	if ok {
		return bytes.NewReader(dataBytes)
	}
	rawData, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	return bytes.NewReader(rawData)
}

func (p *PoetryClient) Do(url, method string, reader io.Reader, header http.Header) ([]byte, error) {
	if !strings.HasPrefix(url, p.PoetryAPIServer) {
		url = p.PoetryAPIServer + url
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}

	req.Header = header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 == 2 {
		return body, nil
	}

	return nil, fmt.Errorf("response status error: %s %s %v %d", url, string(body), resp.Header, resp.StatusCode)
}

func (p *PoetryClient) Deserialize(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func (p *PoetryClient) GetUserPermissionUUIDs(roleID int64, productName string, log *xlog.Logger) ([]string, error) {
	permissionUUIDs := make([]string, 0)
	rolePermissions, err := p.ListRolePermissions(roleID, ProjectType, productName, log)
	if err != nil {
		log.Errorf("GetUserPermission ListRolePermissions error: %v", err)
		return []string{}, fmt.Errorf("GetUserPermission ListRolePermissions error: %v", err)
	}
	for _, rolePermission := range rolePermissions {
		permissionUUIDs = append(permissionUUIDs, rolePermission.PermissionUUID)
	}
	return permissionUUIDs, nil
}

type RolePermission struct {
	RoleID         int64  `json:"roleId"`
	PermissionUUID string `json:"permissionUUID"`
}

type RolePermissionModels struct {
	RolePermissions []*RolePermission `json:"RolePermissions"`
}

func (p *PoetryClient) ListRolePermissions(roleID int64, roleType int, productName string, log *xlog.Logger) ([]*RolePermission, error) {
	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, p.ApiRootKey))

	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/rolePermission?roleType=%d&roleId=%d&productName=%s", p.PoetryAPIServer, roleType, roleID, productName), "GET", "", header)
	if err != nil {
		log.Errorf("ListRolePermissions SendRequest error: %v", err)
		return nil, fmt.Errorf("ListRolePermissions SendRequest error: %v", err)
	}
	var RolePermissionModels RolePermissionModels
	err = p.Deserialize([]byte(responseBody), &RolePermissionModels.RolePermissions)
	if err != nil {
		log.Errorf("ListRolePermissions Deserialize error: %v", err)
		return nil, fmt.Errorf("ListRolePermissions Deserialize error: %v", err)
	}
	return RolePermissionModels.RolePermissions, nil
}
