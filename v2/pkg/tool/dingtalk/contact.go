/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dingtalk

import "github.com/pkg/errors"

type UserIDResponse struct {
	UserID string `json:"userid"`
}

func (c *Client) GetUserIDByMobile(mobile string) (resp *UserIDResponse, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]string{
		"mobile": mobile,
	}).SetSuccessResult(&resp).
		Post("https://oapi.dingtalk.com/topapi/v2/user/getbymobile")
	return
}

type SubDepartmentInfo struct {
	ID   int64  `json:"dept_id"`
	Name string `json:"name"`
}

func (c *Client) GetSubDepartmentsInfo(id int) (resp []SubDepartmentInfo, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]interface{}{
		"dept_id": id,
	}).SetSuccessResult(&resp).
		Post("https://oapi.dingtalk.com/topapi/v2/department/listsub")
	return
}

type GetDepartmentUserIDResponse struct {
	UserIDList []string `json:"userid_list"`
}

func (c *Client) GetDepartmentUserIDs(id int) (resp *GetDepartmentUserIDResponse, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]interface{}{
		"dept_id": id,
	}).SetSuccessResult(&resp).
		Post("https://oapi.dingtalk.com/topapi/user/listid")
	return
}

type UserInfo struct {
	UserID           string `json:"userid"`
	UnionID          string `json:"unionid"`
	Name             string `json:"name"`
	Avatar           string `json:"avatar"`
	StateCode        string `json:"state_code"`
	ManegerUserID    string `json:"manager_userid"`
	Mobile           string `json:"mobile"`
	Telephone        string `json:"telephone"`
	JobNumber        string `json:"job_number"`
	Title            string `json:"title"`
	Email            string `json:"email"`
	ExclusiveAccount bool   `json:"exclusive_account"`
}

func (c *Client) GetUserInfo(id string) (resp *UserInfo, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]interface{}{
		"userid": id,
	}).SetSuccessResult(&resp).Post("https://oapi.dingtalk.com/topapi/v2/user/get")
	return
}

func (c *Client) GetUserInfos(ids []string) ([]*UserInfo, error) {
	var resp []*UserInfo
	for _, id := range ids {
		if userInfo := c.getUserInfoFromCache(id); userInfo != nil {
			resp = append(resp, userInfo)
			continue
		}
		userInfo, err := c.GetUserInfo(id)
		if err != nil {
			return nil, errors.Wrap(err, "get user infos failed")
		}
		resp = append(resp, userInfo)
		c.storeUserInfoInCache(id, userInfo)
	}
	return resp, nil
}
