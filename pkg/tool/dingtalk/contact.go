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

//func (c *Client) GetUserIDByPhone(phone string) (string, error) {
//	resp, err := c.R().SetBodyJsonMarshal(map[string]string{
//		"mobile": phone,
//	}).Post("https://oapi.dingtalk.com/topapi/v2/user/getbymobile")
//	if err != nil {
//		return "", err
//	}
//	if resp.IsErrorState() {
//		return "", errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
//	}
//	return gjson.Get(resp.String(), "result.userid").String(), nil
//}

type UserIDResp struct {
	UserID string `json:"userid"`
}

func (c *Client) GetUserIDByPhone(phone string) (resp *UserIDResp, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]string{
		"mobile": phone,
	}).SetSuccessResult(&resp).
		Post("https://oapi.dingtalk.com/topapi/v2/user/getbymobile")
	return
}

type SubDepartmentInfoResponse struct {
}

type SubDepartmentInfo struct {
}

func (c *Client) GetSubDepartmentInfo(id int) (resp *SubDepartmentInfoResponse, err error) {
	_, err = c.R().SetBodyJsonMarshal(map[string]interface{}{
		"dept_id": id,
	}).SetSuccessResult(&resp).
		Post("https://oapi.dingtalk.com/topapi/v2/department/listsub")
	return
}
