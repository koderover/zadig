/*
 * Copyright 2024 The KodeRover Authors.
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

package workwx

import "fmt"

const (
	getAccessTokenAPI     = "cgi-bin/gettoken"
	listDepartmentAPI     = "cgi-bin/department/list"
	listDepartmentUserAPI = "cgi-bin/user/simplelist"
	getUserIDByPhoneAPI   = "cgi-bin/user/getuserid"
)

type ApprovalRel int

const (
	ApprovalRelAnd ApprovalRel = 1
	ApprovalRelOr  ApprovalRel = 2
)

type generalResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

const (
	errCodeOK = 0
)

func (r *generalResponse) ToError() error {
	if r.ErrCode != errCodeOK {
		return fmt.Errorf(r.ErrMsg)
	}
	return nil
}

type getAccessTokenResp struct {
	generalResponse `json:"inline"`

	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type ListDepartmentResp struct {
	generalResponse

	Department []*Department `json:"department"`
}

type Department struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	NameEN           string   `json:"name_en"`
	DepartmentLeader []string `json:"department_leader"`
	ParentID         int      `json:"parentid"`
	Order            int      `json:"order"`
}

type ListDepartmentUserResp struct {
	generalResponse `json:"inline"`

	UserList []*UserBriefInfo `json:"user_list"`
}

type UserBriefInfo struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Department []int  `json:"department"`
	OpenUserID string `json:"open_userid"`
}

type FindUserByPhoneResp struct {
	generalResponse `json:"inline"`

	UserID string `json:"userid"`
}
