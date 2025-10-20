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

	"github.com/koderover/zadig/v2/pkg/setting"
)

func (client *Client) GetUserIDByEmailOrMobile(_type, value, userIDType string) (*larkcontact.UserContactInfo, error) {
	if value == "" {
		return nil, errors.New("empty value")
	}
	var body *larkcontact.BatchGetIdUserReqBody
	switch _type {
	case QueryTypeMobile:
		body = larkcontact.NewBatchGetIdUserReqBodyBuilder().Mobiles([]string{value}).Build()
	case QueryTypeEmail:
		body = larkcontact.NewBatchGetIdUserReqBodyBuilder().Emails([]string{value}).Build()
	default:
		return nil, errors.New("invalid query type")
	}
	req := larkcontact.NewBatchGetIdUserReqBuilder().
		UserIdType(userIDType).
		Body(body).
		Build()

	resp, err := client.Contact.User.BatchGetId(context.Background(), req)

	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, resp.CodeError
	}
	if len(resp.Data.UserList) == 0 || resp.Data.UserList[0].UserId == nil {
		return nil, errors.New("user not found")
	}

	return resp.Data.UserList[0], nil
}

// ListAppContactRange get users, departments, groups open id authorized by the lark app
func (client *Client) ListAppContactRange(userIDType string) (*ContactRange, error) {
	cr := &ContactRange{}
	pageToken := ""
	for {
		result, page, err := client.listAppContactRangeWithPage(userIDType, pageToken, defaultPageSize)
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

func (client *Client) listAppContactRangeWithPage(userIDType, pageToken string, size int) (*ContactRange, *pageInfo, error) {
	req := larkcontact.NewListScopeReqBuilder().
		UserIdType(userIDType).
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

func (client *Client) ListSubDepartmentsInfo(departmentID, departmentIDType, userIDType string, fetchChild bool) ([]*DepartmentInfo, error) {
	var list []*DepartmentInfo
	pageToken := ""
	for {
		resp, page, err := client.listSubDepartmentsInfoWithPage(departmentID, departmentIDType, userIDType, pageToken, defaultPageSize, fetchChild)
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

func (client *Client) listSubDepartmentsInfoWithPage(departmentID, departmentIDType, userIDType, pageToken string, size int, fetchChild bool) ([]*DepartmentInfo, *pageInfo, error) {
	req := larkcontact.NewChildrenDepartmentReqBuilder().
		DepartmentId(departmentID).
		UserIdType(userIDType).
		DepartmentIdType(departmentIDType).
		FetchChild(fetchChild).
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
			ID:           getStringFromPointer(item.OpenDepartmentId),
			DepartmentID: getStringFromPointer(item.DepartmentId),
			Name:         getStringFromPointer(item.Name),
		})
	}
	return list, getPageInfo(resp.Data.HasMore, resp.Data.PageToken), nil
}

func (client *Client) ListUserFromDepartment(departmentID, departmentIDType, userIDType string) ([]*UserInfo, error) {
	var list []*UserInfo
	pageToken := ""
	for {
		resp, page, err := client.listUserFromDepartmentWithPage(departmentID, departmentIDType, userIDType, pageToken, defaultPageSize)
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

func (client *Client) listUserFromDepartmentWithPage(departmentID, departmentIDType, userIDType, pageToken string, size int) ([]*UserInfo, *pageInfo, error) {
	req := larkcontact.NewFindByDepartmentUserReqBuilder().
		UserIdType(userIDType).
		DepartmentIdType(departmentIDType).
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
		var userID string
		switch userIDType {
		case setting.LarkUserOpenID:
			userID = getStringFromPointer(item.OpenId)
		case setting.LarkUserID:
			userID = getStringFromPointer(item.UserId)
		default:
			// default to open_id
			userID = getStringFromPointer(item.OpenId)
		}
		list = append(list, &UserInfo{
			ID:     userID,
			Name:   getStringFromPointer(item.Name),
			Avatar: getStringFromPointer(item.Avatar.Avatar240),
		})
	}
	return list, getPageInfo(resp.Data.HasMore, resp.Data.PageToken), nil
}

func (client *Client) GetUserInfoByID(id, userIDType string) (*UserInfo, error) {
	req := larkcontact.NewGetUserReqBuilder().
		UserId(id).
		UserIdType(userIDType).
		DepartmentIdType(setting.LarkDepartmentOpenID).
		Build()

	resp, err := client.Contact.User.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, resp.CodeError
	}

	if resp.Data.User == nil {
		return nil, errors.New("GetUserInfoByID: user is nil")
	}

	avatar := ""
	if resp.Data.User.Avatar != nil {
		avatar = getStringFromPointer(resp.Data.User.Avatar.Avatar240)
	}

	return &UserInfo{
		ID:     id,
		Name:   getStringFromPointer(resp.Data.User.Name),
		Avatar: avatar,
	}, nil
}

func (client *Client) GetDepartmentInfoByID(id, userIDType string) (*DepartmentInfo, error) {
	req := larkcontact.NewGetDepartmentReqBuilder().
		DepartmentId(id).
		UserIdType(userIDType).
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

func (client *Client) GetUserGroup(groupID string) (*larkcontact.Group, error) {
	req := larkcontact.NewGetGroupReqBuilder().GroupId(groupID).Build()
	resp, err := client.Contact.Group.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, resp.CodeError
	}

	return resp.Data.Group, nil
}

func (client *Client) GetUserGroups(_type int, pageToken string) ([]*larkcontact.Group, string, bool, error) {
	req := larkcontact.NewSimplelistGroupReqBuilder().Type(_type).PageSize(100).PageToken(pageToken).Build()
	resp, err := client.Contact.Group.Simplelist(context.Background(), req)
	if err != nil {
		return nil, "", false, err
	}
	if !resp.Success() {
		return nil, "", false, resp.CodeError
	}

	return resp.Data.Grouplist, *resp.Data.PageToken, *resp.Data.HasMore, nil
}

func (client *Client) GetUserGroupMembers(groupID, memberType, memberIDType, pageToken string) ([]*larkcontact.Memberlist, string, bool, error) {
	req := larkcontact.NewSimplelistGroupMemberReqBuilder().GroupId(groupID).MemberType(memberType).MemberIdType(memberIDType).PageSize(100).PageToken(pageToken).Build()
	resp, err := client.Contact.GroupMember.Simplelist(context.Background(), req)
	if err != nil {
		return nil, "", false, err
	}
	if !resp.Success() {
		return nil, "", false, resp.CodeError
	}

	return resp.Data.Memberlist, *resp.Data.PageToken, *resp.Data.HasMore, nil
}
