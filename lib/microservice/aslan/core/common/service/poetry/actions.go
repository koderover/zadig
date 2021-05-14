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

package poetry

import (
	"fmt"

	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func (p *PoetryClient) DeleteEnvRolePermission(productName, envName string, log *xlog.Logger) (int, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleEnv?productName=%s&envName=%s", p.PoetryAPIServer, productName, envName), "DELETE", nil, p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("DeleteEnvRolePermission SendRequest error: %v", err)
		return 0, e.ErrListUsers.AddDesc(err.Error())
	}
	responseMessage := &ResponseMessage{}
	err = p.Deserialize([]byte(responseBody), responseMessage)
	if err != nil {
		log.Errorf("DeleteEnvRolePermission error: %v", err)
		return 0, e.ErrListUsers.AddDesc(err.Error())
	}
	if responseMessage.ResultCode == 0 {
		return 1, nil
	}
	return 0, e.ErrListUsers.AddDesc(err.Error())
}

func (p *PoetryClient) ListEnvRolePermission(productName, envName string, roleID int64, log *xlog.Logger) ([]*EnvRolePermission, error) {
	envRolePermissions := make([]*EnvRolePermission, 0)
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleEnv?productName=%s&envName=%s&roleId=%d", p.PoetryAPIServer, productName, envName, roleID), "GET", nil, p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("GetEnvRolePermission SendRequest error: %v", err)
		return envRolePermissions, e.ErrListUsers.AddDesc(err.Error())
	}

	err = p.Deserialize([]byte(responseBody), &envRolePermissions)
	if err != nil {
		log.Errorf("GetEnvRolePermission error: %v", err)
		return envRolePermissions, e.ErrListUsers.AddDesc(err.Error())
	}
	return envRolePermissions, nil
}

func (p *PoetryClient) AddProductTeam(productName string, teamID int, userIDs []int, log *xlog.Logger) (int, error) {
	body := make(map[string]interface{})
	body["productName"] = productName
	body["teamId"] = teamID
	body["userIds"] = userIDs

	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/teamProduct", p.PoetryAPIServer), "POST", body, p.GetRootTokenHeader())
	var ResponseMessage ResponseMessage
	err = p.Deserialize([]byte(responseBody), &ResponseMessage)
	if err != nil {
		log.Errorf("AddProductTeam error: %v", err)
		return 0, e.ErrCreateProductTeam.AddDesc(err.Error())
	}
	if ResponseMessage.ResultCode == 0 {
		return 1, nil
	}
	return 0, e.ErrCreateProductTeam.AddDesc(err.Error())
}

func (p *PoetryClient) DeleteProductTeam(productName string, log *xlog.Logger) error {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/teamProduct?productName=%s", p.PoetryAPIServer, productName), "DELETE", nil, p.GetRootTokenHeader())
	var ResponseMessage ResponseMessage
	err = p.Deserialize([]byte(responseBody), &ResponseMessage)
	if err != nil {
		log.Errorf("DeleteProductTeam error: %v", err)
		return e.ErrDeleteProductTeam.AddDesc(err.Error())
	}
	if ResponseMessage.ResultCode == 0 {
		return nil
	}
	return e.ErrDeleteProductTeam.AddDesc(err.Error())
}

func (p *PoetryClient) HasOperatePermission(productName, permissionUUID string, userID int, isSuperUser bool, log *xlog.Logger) bool {
	if isSuperUser {
		return true
	}
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/userPermissionRelation?userId=%d&permissionUUID=%s&permissionType=%d&productName=%s", p.PoetryAPIServer, userID, permissionUUID, 2, productName), "GET", nil, p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("userPermissionRelation request error: %v", err)
		return false
	}
	rolePermissionMap := make(map[string]bool)
	err = p.Deserialize([]byte(responseBody), &rolePermissionMap)
	if err != nil {
		log.Errorf("userPermissionRelation Deserialize error: %v", err)
		return false
	}

	return rolePermissionMap["isContain"]
}
