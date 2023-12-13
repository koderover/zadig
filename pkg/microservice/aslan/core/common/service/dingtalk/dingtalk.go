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

import (
	"context"
	"strconv"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
)

type SubDepartmentAndUserIDsResponse struct {
	SubDepartmentIDs []*DepartmentInfo `json:"sub_department_list"`
	Users            []*UserInfo       `json:"user_list"`
}

type DepartmentInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func GetDingTalkDepartment(id, departmentID string) (*SubDepartmentAndUserIDsResponse, error) {
	client, err := GetDingTalkClientByIMAppID(id)
	if err != nil {
		return nil, errors.Wrap(err, "get dingtalk client error")
	}
	deptID, err := strconv.Atoi(departmentID)
	if err != nil {
		return nil, errors.Wrap(err, "parse departmentID error")
	}
	userIDsResp, err := client.GetDepartmentUserIDs(deptID)
	if err != nil {
		return nil, errors.Wrap(err, "get department userIDsResp error")
	}

	userInfos, err := client.GetUserInfos(userIDsResp.UserIDList)
	if err != nil {
		return nil, errors.Wrap(err, "get user infos error")
	}

	subDepartments, err := client.GetSubDepartmentsInfo(deptID)
	if err != nil {
		return nil, errors.Wrap(err, "get sub departments error")
	}
	return &SubDepartmentAndUserIDsResponse{
		SubDepartmentIDs: func() (resp []*DepartmentInfo) {
			for _, department := range subDepartments {
				resp = append(resp, &DepartmentInfo{
					ID:   strconv.FormatInt(department.ID, 10),
					Name: department.Name,
				})
			}
			return
		}(),
		Users: func() (resp []*UserInfo) {
			for _, info := range userInfos {
				resp = append(resp, &UserInfo{
					ID:     info.UserID,
					Name:   info.Name,
					Avatar: info.Avatar,
				})
			}
			return
		}(),
	}, nil
}

func GetDingTalkUserIDByMobile(id, mobile string) (string, error) {
	client, err := GetDingTalkClientByIMAppID(id)
	if err != nil {
		return "", errors.Wrap(err, "get dingtalk client error")
	}
	resp, err := client.GetUserIDByMobile(mobile)
	if err != nil {
		return "", errors.Wrap(err, "get userID by mobile error")
	}
	return resp.UserID, nil
}

func GetDingTalkClientByIMAppID(id string) (*dingtalk.Client, error) {
	imApp, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "db error")
	}
	if imApp.Type != setting.IMDingTalk {
		return nil, errors.Errorf("unexpected imApp type %s", imApp.Type)
	}
	return dingtalk.NewClient(imApp.DingTalkAppKey, imApp.DingTalkAppSecret), nil
}
