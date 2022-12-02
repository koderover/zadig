/*
 * Copyright 2022 The KodeRover Authors.
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

package service

import (
	"context"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/lark"
)

type DepartmentInfo struct {
	//Name              string `json:"name,omitempty"`
	UserList          []*lark.UserInfo
	SubDepartmentList []*lark.DepartmentInfo
}

func GetLarkDepartment(approvalID, openID string) (*DepartmentInfo, error) {
	cli, err := getLarkClientByExternalApprovalID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	userList, err := cli.ListUserFromDepartment(openID)
	if err != nil {
		return nil, errors.Wrap(err, "get user list")
	}
	departmentList, err := cli.ListSubDepartmentsInfo(openID)
	if err != nil {
		return nil, errors.Wrap(err, "get sub-department list")
	}
	return &DepartmentInfo{
		UserList:          userList,
		SubDepartmentList: departmentList,
	}, nil
}

func GetLarkAppContactRange(approvalID string) (*DepartmentInfo, error) {
	cli, err := getLarkClientByExternalApprovalID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	reply, err := cli.ListAppContactRange()
	if err != nil {
		return nil, errors.Wrap(err, "list range")
	}

	var userList []*lark.UserInfo
	for _, userID := range reply.UserIDs {
		info, err := cli.GetUserInfoByID(userID)
		if err != nil {
			return nil, errors.Wrap(err, "get user info")
		}
		userList = append(userList, info)
	}
	departmentList, err := getLarkDepartmentInfoConcurrently(cli, reply.DepartmentIDs, 10)
	if err != nil {
		return nil, errors.Wrap(err, "get department info")
	}
	return &DepartmentInfo{
		UserList:          userList,
		SubDepartmentList: departmentList,
	}, nil
}

func getLarkClientByExternalApprovalID(id string) (*lark.Client, error) {
	approval, err := mongodb.NewExternalApprovalColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "get external approval data")
	}
	if approval.Type != setting.IMLark {
		return nil, errors.Errorf("unexpected approval type %s", approval.Type)
	}
	return lark.NewClient(approval.AppID, approval.AppSecret), nil
}

func getLarkDepartmentInfoConcurrently(client *lark.Client, idList []string, concurrentNum int) ([]*lark.DepartmentInfo, error) {
	var reply []*lark.DepartmentInfo
	idNum := len(idList)

	type result struct {
		*lark.DepartmentInfo
		Err error
	}
	argCh := make(chan string, 100)
	resultCh := make(chan *result, 100)
	for i := 0; i < concurrentNum; i++ {
		go func() {
			for arg := range argCh {
				info, err := client.GetDepartmentInfoByID(arg)
				resultCh <- &result{
					DepartmentInfo: info,
					Err:            err,
				}
			}
		}()
	}
	for _, s := range idList {
		argCh <- s
	}
	close(argCh)

	for i := 0; i < idNum; i++ {
		re := <-resultCh
		if re.Err != nil {
			return nil, errors.Wrap(re.Err, "get department id list concurrently")
		}
		reply = append(reply, re.DepartmentInfo)
	}
	return reply, nil
}

func GetLarkUserID(approvalID, queryType, queryValue string) (string, error) {
	switch queryType {
	case "email":
	default:
		return "", errors.New("invalid query type")
	}

	cli, err := getLarkClientByExternalApprovalID(approvalID)
	if err != nil {
		return "", errors.Wrap(err, "get client")
	}
	return cli.GetUserOpenIDByEmail(queryValue)
}
