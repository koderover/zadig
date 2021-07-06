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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/types/permission"
)

//roleType
const (
	ProjectType      = 2
	AllUsersRoleName = "all-users"
)

type UserRole struct {
	RoleID      int64  `json:"roleId"`
	UserID      int    `json:"userId"`
	ProductName string `json:"productName"`
}

type RoleProduct struct {
	RoleID      int64  `json:"roleId"`
	ProductName string `json:"productName"`
	Role        Role   `json:"role"`
}

type UpdateUserRoleReq struct {
	ProductName string `json:"productName"`
	RoleID      int64  `json:"roleId"`
	RoleType    int    `json:"roleType"`
	UserIDList  []int  `json:"userIds"`
}

type RolePermission struct {
	RoleID         int64  `json:"roleId"`
	PermissionUUID string `json:"permissionUUID"`
}

type RoleEnv struct {
	RoleID      int    `json:"roleId"`
	EnvName     string `json:"envName"`
	ProductName string `json:"productName"`
}

type RolePermissionReq struct {
	RoleID         int64    `json:"roleId"`
	PermissionUUID []string `json:"permissionUUIDs"`
	RoleType       int      `json:"roleType"`
	ProductName    string   `json:"productName"`
}

type Role struct {
	ID          int64  `json:"id"`
	IsDisabled  bool   `json:"isDisabled"`
	Name        string `json:"name"`
	ProductName string `json:"productName"`
	RoleType    int    `json:"roleType"`
	UpdateBy    string `json:"updateBy"`
	UpdateAt    int64  `json:"updatedAt"`
}

type UserEnv struct {
	ProductName    string `json:"productName"`
	EnvName        string `json:"envName"`
	UserID         int    `json:"userId"`
	PermissionUUID string `json:"permissionUUID"`
}

func (c *Client) ListUserRoles(userID, roleType int, log *zap.SugaredLogger) ([]*UserRole, error) {
	url := "/directory/roleUser"

	ur := make([]*UserRole, 0)
	_, err := c.Get(url, httpclient.SetResult(&ur), httpclient.SetQueryParam("roleType", strconv.Itoa(roleType)), httpclient.SetQueryParam("userId", strconv.Itoa(userID)))
	if err != nil {
		log.Errorf("ListUserRoles error: %v", err)
		return nil, err
	}

	return ur, nil
}

func (c *Client) ListPermissionUsers(productName string, roleID int64, roleType int, log *zap.SugaredLogger) ([]int, error) {
	url := "/directory/roleUser"
	qs := map[string]string{
		"productName": productName,
		"roleType":    strconv.Itoa(roleType),
		"roleId":      strconv.Itoa(int(roleID)),
	}

	ur := make([]*UserRole, 0)
	_, err := c.Get(url, httpclient.SetResult(&ur), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListPermissionUsers error: %v", err)
		return nil, err
	}

	var ret []int
	for _, userRole := range ur {
		ret = append(ret, userRole.UserID)
	}
	return ret, nil
}

func (c *Client) UpdateUserRole(roleID int64, roleType int, productName string, userIDList []int, log *zap.SugaredLogger) error {
	url := "/directory/roleUser"
	req := &UpdateUserRoleReq{
		ProductName: productName,
		RoleID:      roleID,
		RoleType:    roleType,
		UserIDList:  userIDList,
	}

	_, err := c.Post(url, httpclient.SetBody(req))
	if err != nil {
		log.Errorf("UpdateUserRole failed, error: %v", err)
		return err
	}

	return nil
}

func (c *Client) DeleteUserRole(roleID int64, roleType, userID int, productName string, log *zap.SugaredLogger) error {
	url := "/directory/roleUser"
	qs := map[string]string{
		"productName": productName,
		"roleType":    strconv.Itoa(roleType),
		"roleId":      strconv.Itoa(int(roleID)),
		"userId":      strconv.Itoa(userID),
	}

	_, err := c.Delete(url, httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("DeleteUserRole error: %v", err)
		return err
	}
	return nil
}

func (c *Client) ListRoleProjects(roleID int64, log *zap.SugaredLogger) ([]*RoleProduct, error) {
	url := "/directory/roleProduct"

	rp := make([]*RoleProduct, 0)
	_, err := c.Get(url, httpclient.SetResult(&rp), httpclient.SetQueryParam("roleId", strconv.Itoa(int(roleID))))
	if err != nil {
		log.Errorf("ListRoleProjects error: %v", err)
		return nil, err
	}

	return rp, nil
}

func (c *Client) ContributorRoleExist(productName string, log *zap.SugaredLogger) bool {
	url := "/directory/roleProduct"

	rp := make([]*RoleProduct, 0)
	_, err := c.Get(url, httpclient.SetResult(&rp), httpclient.SetQueryParam("productName", productName))
	if err != nil {
		log.Errorf("Failed to list role project, error: %v", err)
		return false
	}

	for _, roleProduct := range rp {
		if roleProduct.Role.Name == setting.RoleContributor {
			return true
		}
	}
	return false
}

func (c *Client) GetContributorRoleID(productName string, log *zap.SugaredLogger) int64 {
	url := "/directory/roleProduct"

	rp := make([]*RoleProduct, 0)
	_, err := c.Get(url, httpclient.SetResult(&rp), httpclient.SetQueryParam("productName", productName))
	if err != nil {
		log.Errorf("Failed to list role project, sendRequest error: %v", err)
		return -1
	}

	for _, roleProduct := range rp {
		if roleProduct.Role.Name == setting.RoleContributor {
			return roleProduct.RoleID
		}
	}
	return -1
}

func (c *Client) ListRolePermissions(roleID int64, roleType int, productName string, log *zap.SugaredLogger) ([]*RolePermission, error) {
	url := "/directory/rolePermission"
	qs := map[string]string{
		"productName": productName,
		"roleType":    strconv.Itoa(roleType),
		"roleId":      strconv.Itoa(int(roleID)),
	}

	rp := make([]*RolePermission, 0)
	_, err := c.Get(url, httpclient.SetResult(&rp), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListRolePermissions error: %v", err)
		return nil, err
	}

	return rp, nil
}

func (c *Client) ListRoleEnvs(productName, envName string, roleID int64, log *zap.SugaredLogger) ([]*RoleEnv, error) {
	url := "/directory/roleEnv"
	qs := map[string]string{
		"productName": productName,
		"envName":     envName,
		"roleId":      strconv.Itoa(int(roleID)),
	}

	re := make([]*RoleEnv, 0)
	_, err := c.Get(url, httpclient.SetResult(&re), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListRoleEnvs error: %v", err)
		return nil, err
	}

	return re, nil
}

func (c *Client) GetUserProject(userID int, log *zap.SugaredLogger) (map[string][]int64, error) {
	productNameMap := make(map[string][]int64)
	userRoles, err := c.ListUserRoles(userID, ProjectType, log)
	if err != nil {
		log.Errorf("GetUserProject ListUserRoles error: %v", err)
		return productNameMap, fmt.Errorf("GetUserProject ListUserRoles error: %v", err)
	}
	for _, userRole := range userRoles {
		productNameMap[userRole.ProductName] = append(productNameMap[userRole.ProductName], userRole.RoleID)
	}
	return productNameMap, nil
}

func (c *Client) GetUserPermissionUUIDs(roleID int64, productName string, log *zap.SugaredLogger) ([]string, error) {
	permissionUUIDs := make([]string, 0)
	rolePermissions, err := c.ListRolePermissions(roleID, ProjectType, productName, log)
	if err != nil {
		log.Errorf("GetUserPermission ListRolePermissions error: %v", err)
		return []string{}, fmt.Errorf("GetUserPermission ListRolePermissions error: %v", err)
	}
	for _, rolePermission := range rolePermissions {
		permissionUUIDs = append(permissionUUIDs, rolePermission.PermissionUUID)
	}
	return permissionUUIDs, nil
}

// ListProductPermissionUsers productName和permissionId为空时只返回所有管理员用户，不为空则返回所有管理员用户和有该权限的普通用户
func (c *Client) ListProductPermissionUsers(productName, permissionID string, log *zap.SugaredLogger) ([]string, error) {
	url := "/directory/productUser"
	qs := map[string]string{
		"productName":  productName,
		"permissionId": permissionID,
	}

	resp := make([]string, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListProductPermissionUsers error: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) CreateContributorRole(product string, log *zap.SugaredLogger) error {
	// create role first
	url := "/directory/roles"
	req := &Role{
		IsDisabled:  false,
		Name:        setting.RoleContributor,
		ProductName: product,
		RoleType:    ProjectType,
		UpdateBy:    setting.SystemUser,
		UpdateAt:    time.Now().Unix(),
	}

	r := &Role{}
	_, err := c.Post(url, httpclient.SetBody(req), httpclient.SetResult(r))
	if err != nil {
		log.Errorf("CreateContributorRole failed, error: %v", err)
		return err
	}

	// 然后创建role product
	roleProductURL := "/directory/roleProduct"
	roleProductReq := RoleProduct{
		RoleID:      r.ID,
		ProductName: product,
	}

	_, err = c.Post(roleProductURL, httpclient.SetBody(roleProductReq))
	if err != nil {
		log.Errorf("CreateContributorRole failed, error: %v", err)
		return err
	}
	permissionList := []string{
		permission.WorkflowTaskUUID,
		permission.WorkflowListUUID,
		permission.TestEnvDeleteUUID,
		permission.TestEnvManageUUID,
		permission.TestEnvListUUID,
		permission.TestUpdateEnvUUID,
		permission.BuildListUUID,
		permission.TestListUUID,
		permission.ServiceTemplateListUUID,
	}
	permissionReq := RolePermissionReq{
		RoleID:         r.ID,
		RoleType:       r.RoleType,
		PermissionUUID: permissionList,
		ProductName:    product,
	}
	rolePermissionURL := "/directory/rolePermission"

	_, err = c.Post(rolePermissionURL, httpclient.SetBody(permissionReq))
	if err != nil {
		log.Errorf("CreateContributorRole failed, error: %v", err)
		return err
	}

	return nil
}

// ListRoles 根据项目里面的角色名称获取对应的角色ID
func (c *Client) ListRoles(productName string, log *zap.SugaredLogger) (*Role, error) {
	url := "/directory/roles"
	qs := map[string]string{
		"name":        AllUsersRoleName,
		"productName": productName,
		"roleType":    strconv.Itoa(ProjectType),
	}

	resp := make([]*Role, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListRoles error: %v", err)
		return nil, err
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("ListRoles Not Found")
	}

	return resp[0], nil
}

func (c *Client) ListUserEnvPermission(productName string, userID int, log *zap.SugaredLogger) ([]*UserEnv, error) {
	url := "/directory/userEnvPermission"
	qs := map[string]string{
		"productName": productName,
		"userId":      strconv.Itoa(userID),
	}

	resp := make([]*UserEnv, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp), httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("ListUserEnvPermission error: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) DeleteUserEnvPermission(productName, username string, userID int, log *zap.SugaredLogger) error {
	url := "/directory/userEnvPermission"
	qs := map[string]string{
		"productName": productName,
		"envName":     strings.ToLower(username),
		"userId":      strconv.Itoa(userID),
	}

	_, err := c.Delete(url, httpclient.SetQueryParams(qs))
	if err != nil {
		log.Errorf("DeleteUserEnvPermission error: %v", err)
		return err
	}

	return nil
}

func (c *Client) GetUserEnvPermission(userID int, log *zap.SugaredLogger) ([]*UserEnv, error) {
	url := "/directory/userEnvPermission"

	resp := make([]*UserEnv, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp), httpclient.SetQueryParam("userId", strconv.Itoa(userID)))
	if err != nil {
		log.Errorf("UserEnvPermission request error: %v", err)
		return nil, err
	}

	return resp, nil
}
