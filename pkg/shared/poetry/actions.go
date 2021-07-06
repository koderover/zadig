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
	"strconv"

	"go.uber.org/zap"

	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) AddEnvRolePermission(productName, envName string, permissionUUIDs []string, roleID int, log *zap.SugaredLogger) (int, error) {
	url := "/directory/roleEnv"
	req := map[string]interface{}{
		"productName":     productName,
		"envName":         envName,
		"permissionUUIDs": permissionUUIDs,
		"roleId":          roleID,
	}

	rm := &ResponseMessage{}
	_, err := c.Post(url, httpclient.SetBody(req), httpclient.SetResult(rm))
	if err != nil {
		log.Errorf("AddEnvRolePermission error: %v", err)
		return 0, e.ErrListUsers.AddDesc(err.Error())
	}

	if rm.ResultCode == 0 {
		return 1, nil
	}

	return 0, e.ErrListUsers.AddDesc(fmt.Sprintf("ResultCode: %d", rm.ResultCode))
}

func (c *Client) DeleteEnvRolePermission(productName, envName string, log *zap.SugaredLogger) (int, error) {
	url := "/directory/roleEnv"
	qs := map[string]string{
		"productName": productName,
		"envName":     envName,
	}

	rm := &ResponseMessage{}
	_, err := c.Delete(url, httpclient.SetQueryParams(qs), httpclient.SetResult(rm))
	if err != nil {
		log.Errorf("DeleteEnvRolePermission error: %v", err)
		return 0, e.ErrListUsers.AddDesc(err.Error())
	}

	if rm.ResultCode == 0 {
		return 1, nil
	}
	return 0, e.ErrListUsers.AddDesc(fmt.Sprintf("ResultCode: %d", rm.ResultCode))
}

func (c *Client) ListEnvRolePermission(productName, envName string, roleID int64, log *zap.SugaredLogger) ([]*EnvRolePermission, error) {
	url := "/directory/roleEnv"
	qs := map[string]string{
		"productName": productName,
		"envName":     envName,
		"roleId":      strconv.Itoa(int(roleID)),
	}

	res := make([]*EnvRolePermission, 0)
	_, err := c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&res))
	if err != nil {
		log.Errorf("GetEnvRolePermission error: %v", err)
		return res, e.ErrListUsers.AddDesc(err.Error())
	}

	return res, nil
}

func (c *Client) AddProductTeam(productName string, teamID int, userIDs []int, log *zap.SugaredLogger) (int, error) {
	url := "/directory/teamProduct"
	req := map[string]interface{}{
		"productName": productName,
		"teamId":      teamID,
		"userIds":     userIDs,
	}

	rm := &ResponseMessage{}
	_, err := c.Post(url, httpclient.SetBody(req), httpclient.SetResult(rm))
	if err != nil {
		log.Errorf("AddProductTeam error: %v", err)
		return 0, e.ErrCreateProductTeam.AddDesc(err.Error())
	}

	if rm.ResultCode == 0 {
		return 1, nil
	}
	return 0, e.ErrCreateProductTeam.AddDesc(fmt.Sprintf("ResultCode: %d", rm.ResultCode))
}

func (c *Client) DeleteProductTeam(productName string, log *zap.SugaredLogger) error {
	url := "/directory/teamProduct"

	rm := &ResponseMessage{}
	_, err := c.Delete(url, httpclient.SetQueryParam("productName", productName), httpclient.SetResult(rm))
	if err != nil {
		log.Errorf("DeleteProductTeam error: %v", err)
		return e.ErrDeleteProductTeam.AddDesc(err.Error())
	}
	if rm.ResultCode == 0 {
		return nil
	}
	return e.ErrDeleteProductTeam.AddDesc(fmt.Sprintf("ResultCode: %d", rm.ResultCode))
}

func (c *Client) HasOperatePermission(productName, permissionUUID string, userID int, isSuperUser bool, log *zap.SugaredLogger) bool {
	if isSuperUser {
		return true
	}

	res, err := c.GetUserPermissionUUIDMap(productName, permissionUUID, userID, log)
	if err != nil {
		return false
	}

	return res["isContain"]
}

func (c *Client) GetUserPermissionUUIDMap(productName, permissionUUID string, userID int, log *zap.SugaredLogger) (map[string]bool, error) {
	url := "/directory/userPermissionRelation"
	qs := map[string]string{
		"productName":    productName,
		"userId":         strconv.Itoa(userID),
		"permissionUUID": permissionUUID,
		"permissionType": "2",
	}

	res := make(map[string]bool)
	_, err := c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&res))
	if err != nil {
		log.Errorf("GetUserPermissionUUIDMap error: %v", err)
		return nil, err
	}

	return res, nil
}
