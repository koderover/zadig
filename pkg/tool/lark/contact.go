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

package lark

import (
	"context"

	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/setting"
)

func (client *Client) GetUserOpenIDByEmailOrMobile(_type, value string) (string, error) {
	if value == "" {
		return "", errors.New("empty value")
	}
	var body *larkcontact.BatchGetIdUserReqBody
	switch _type {
	case QueryTypeMobile:
		body = larkcontact.NewBatchGetIdUserReqBodyBuilder().Mobiles([]string{value}).Build()
	case QueryTypeEmail:
		body = larkcontact.NewBatchGetIdUserReqBodyBuilder().Emails([]string{value}).Build()
	default:
		return "", errors.New("invalid query type")
	}
	req := larkcontact.NewBatchGetIdUserReqBuilder().
		UserIdType(setting.LarkUserOpenID).
		Body(body).
		Build()

	resp, err := client.Contact.User.BatchGetId(context.Background(), req)

	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", resp.CodeError
	}
	if len(resp.Data.UserList) == 0 || resp.Data.UserList[0].UserId == nil {
		return "", errors.New("user not found")
	}

	return *resp.Data.UserList[0].UserId, nil
}

// ListAppContactRange get users, departments, groups open id authorized by the lark app
func (client *Client) ListAppContactRange() (*ContactRange, error) {
	cr := &ContactRange{}
	pageToken := ""
	for {
		result, page, err := client.listAppContactRangeWithPage(pageToken, defaultPageSize)
		if err != nil {
			return nil, err
		}

		cr.UserIDs = append(cr.UserIDs, result.UserIDs...)
		cr.DepartmentIDs = append(cr.DepartmentIDs, result.DepartmentIDs...)
		cr.GroupIDs = append(cr.GroupIDs, result.GroupIDs...)

		if !page.hasMore || page.token == "" {
			break
		}
		pageToken = page.token
	}
	return cr, nil
}

func (client *Client) listAppContactRangeWithPage(pageToken string, size int) (*ContactRange, *pageInfo, error) {
	req := larkcontact.NewListScopeReqBuilder().
		UserIdType(setting.LarkUserOpenID).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		PageSize(size).
		PageToken(pageToken).
		Build()

	resp, err := client.Contact.Scope.List(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	if !resp.Success() {
		return nil, nil, resp.CodeError
	}

	return &ContactRange{
		UserIDs:       resp.Data.UserIds,
		DepartmentIDs: resp.Data.DepartmentIds,
		GroupIDs:      resp.Data.GroupIds,
	}, getPageInfo(resp.Data.HasMore, resp.Data.PageToken), nil
}

func (client *Client) ListSubDepartmentsInfo(departmentID string) ([]*DepartmentInfo, error) {
	var list []*DepartmentInfo
	pageToken := ""
	for {
		resp, page, err := client.listSubDepartmentsInfoWithPage(departmentID, pageToken, defaultPageSize)
		if err != nil {
			return nil, err
		}

		list = append(list, resp...)
		if !page.hasMore || page.token == "" {
			break
		}
		pageToken = page.token
	}
	return list, nil
}

func (client *Client) listSubDepartmentsInfoWithPage(departmentID, pageToken string, size int) ([]*DepartmentInfo, *pageInfo, error) {
	req := larkcontact.NewChildrenDepartmentReqBuilder().
		DepartmentId(departmentID).
		UserIdType(setting.LarkUserOpenID).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		FetchChild(false).
		PageSize(size).
		PageToken(pageToken).
		Build()

	resp, err := client.Contact.Department.Children(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	if !resp.Success() {
		return nil, nil, resp.CodeError
	}

	var list []*DepartmentInfo
	for _, item := range resp.Data.Items {
		list = append(list, &DepartmentInfo{
			ID:   getStringFromPointer(item.OpenDepartmentId),
			Name: getStringFromPointer(item.Name),
		})
	}
	return list, getPageInfo(resp.Data.HasMore, resp.Data.PageToken), nil
}

func (client *Client) ListUserFromDepartment(departmentID string) ([]*UserInfo, error) {
	var list []*UserInfo
	pageToken := ""
	for {
		resp, page, err := client.listUserFromDepartmentWithPage(departmentID, pageToken, defaultPageSize)
		if err != nil {
			return nil, err
		}

		list = append(list, resp...)
		if !page.hasMore || page.token == "" {
			break
		}
		pageToken = page.token
	}
	return list, nil
}

func (client *Client) listUserFromDepartmentWithPage(departmentID, pageToken string, size int) ([]*UserInfo, *pageInfo, error) {
	req := larkcontact.NewFindByDepartmentUserReqBuilder().
		UserIdType(setting.LarkUserOpenID).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		DepartmentId(departmentID).
		PageSize(size).
		PageToken(pageToken).
		Build()

	resp, err := client.Contact.User.FindByDepartment(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	if !resp.Success() {
		return nil, nil, resp.CodeError
	}

	var list []*UserInfo
	for _, item := range resp.Data.Items {
		list = append(list, &UserInfo{
			ID:     getStringFromPointer(item.OpenId),
			Name:   getStringFromPointer(item.Name),
			Avatar: getStringFromPointer(item.Avatar.Avatar240),
		})
	}
	return list, getPageInfo(resp.Data.HasMore, resp.Data.PageToken), nil
}

func (client *Client) GetUserInfoByID(id string) (*UserInfo, error) {
	req := larkcontact.NewGetUserReqBuilder().
		UserId(id).
		UserIdType(setting.LarkUserOpenID).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		Build()

	resp, err := client.Contact.User.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, resp.CodeError
	}

	return &UserInfo{
		ID:     id,
		Name:   getStringFromPointer(resp.Data.User.Name),
		Avatar: getStringFromPointer(resp.Data.User.Avatar.Avatar240),
	}, nil
}

func (client *Client) GetDepartmentInfoByID(id string) (*DepartmentInfo, error) {
	req := larkcontact.NewGetDepartmentReqBuilder().
		DepartmentId(id).
		UserIdType(setting.LarkUserOpenID).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		Build()

	resp, err := client.Contact.Department.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, resp.CodeError
	}

	return &DepartmentInfo{
		ID:   id,
		Name: getStringFromPointer(resp.Data.Department.Name),
	}, nil
}
