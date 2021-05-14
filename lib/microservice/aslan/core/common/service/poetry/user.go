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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types/permission"
)

//roleType
const (
	SystemType       = 1
	ProjectType      = 2
	AllUsersRoleName = "all-users"
)

type UserRole struct {
	RoleID      int64  `json:"roleId"`
	UserID      int    `json:"userId"`
	ProductName string `json:"productName"`
}

type UserRoleModels struct {
	UserRoles []*UserRole `json:"userRoles"`
}

//ListUserRoles ...
func (p *PoetryClient) ListUserRoles(userID, roleType int, log *xlog.Logger) ([]*UserRole, error) {
	reponseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleUser?roleType=%d&userId=%d", p.PoetryAPIServer, roleType, userID), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListUserRoles SendRequest error: %v", err)
		return nil, fmt.Errorf("ListUserRoles SendRequest error: %v", err)
	}
	var UserRoleModels UserRoleModels
	err = p.Deserialize([]byte(reponseBody), &UserRoleModels.UserRoles)
	if err != nil {
		log.Errorf("ListUserRoles error: %v", err)
		return nil, fmt.Errorf("ListUserRoles error: %v", err)
	}
	return UserRoleModels.UserRoles, nil
}

func (p *PoetryClient) ListPermissionUsers(productName string, roleID int64, roleType int, log *xlog.Logger) ([]int, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleUser?productName=%s&roleType=%d&roleId=%d", p.PoetryAPIServer, productName, roleType, roleID), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListUsers SendRequest error: %v", err)
		return nil, fmt.Errorf("ListUsers SendRequest error: %v", err)
	}
	var UserRoleModels UserRoleModels
	err = p.Deserialize([]byte(responseBody), &UserRoleModels.UserRoles)
	if err != nil {
		log.Errorf("ListUsers error: %v", err)
		return nil, fmt.Errorf("ListUsers error: %v", err)
	}
	ret := []int{}
	for _, userRole := range UserRoleModels.UserRoles {
		ret = append(ret, userRole.UserID)
	}
	return ret, nil
}

type UpdateUserRoleReq struct {
	ProductName string `json:"productName"`
	RoleID      int64  `json:"roleId"`
	RoleType    int    `json:"roleType"`
	UserIDList  []int  `json:"userIds"`
}

func (p *PoetryClient) UpdateUserRole(roleID int64, roleType int, productName string, userIDList []int, log *xlog.Logger) error {
	header := p.GetRootTokenHeader()
	header.Set("content-type", "application/json")
	req := UpdateUserRoleReq{
		ProductName: productName,
		RoleID:      roleID,
		RoleType:    roleType,
		UserIDList:  userIDList,
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		log.Errorf("UpdateUserRole failed, marshal json error: %v", err)
		return err
	}
	_, err = p.Do("/directory/roleUser", "POST", bytes.NewBuffer(reqBody), header)
	if err != nil {
		log.Errorf("CreateContributorRole failed, call poetry API error: %v", err)
		return err
	}
	return nil
}

func (p *PoetryClient) DeleteUserRole(roleID int64, roleType, userID int, productName string, log *xlog.Logger) error {
	_, err := p.SendRequest(fmt.Sprintf("%s/directory/roleUser?productName=%s&roleType=%d&roleId=%d&userId=%d", p.PoetryAPIServer, productName, roleType, roleID, userID), "DELETE", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("DeleteUserRole SendRequest error: %v", err)
		return fmt.Errorf("DeleteUserRole SendRequest error: %v", err)
	}
	return nil
}

type RoleProduct struct {
	RoleID      int64  `json:"roleId"`
	ProductName string `json:"productName"`
	Role        Role   `json:"role"`
}

type RoleProductModels struct {
	RoleProducts []*RoleProduct `json:"RoleProducts"`
}

//ListRoleProjects ...
func (p *PoetryClient) ListRoleProjects(roleID int64, log *xlog.Logger) ([]*RoleProduct, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleProduct?roleId=%d", p.PoetryAPIServer, roleID), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListRoleProjects SendRequest error: %v", err)
		return nil, fmt.Errorf("ListRoleProjects SendRequest error: %v", err)
	}
	var RoleProductModels RoleProductModels
	err = p.Deserialize([]byte(responseBody), &RoleProductModels.RoleProducts)
	if err != nil {
		log.Errorf("ListRoleProjects Deserialize error: %v", err)
		return nil, fmt.Errorf("ListRoleProjects Deserialize error: %v", err)
	}
	return RoleProductModels.RoleProducts, nil
}

func (p *PoetryClient) ContributorRoleExist(productName string, log *xlog.Logger) bool {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleProduct?productName=%s", p.PoetryAPIServer, productName), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("Failed to list role project, sendRequest error: %v", err)
		return false
	}
	var RoleProductModels RoleProductModels
	err = p.Deserialize([]byte(responseBody), &RoleProductModels.RoleProducts)
	if err != nil {
		log.Errorf("FindContributorRole Deserialize error: %v", err)
		return false
	}
	for _, roleProduct := range RoleProductModels.RoleProducts {
		if roleProduct.Role.Name == setting.RoleContributor {
			return true
		}
	}
	return false
}

func (p *PoetryClient) GetContributorRoleID(productName string, log *xlog.Logger) int64 {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleProduct?productName=%s", p.PoetryAPIServer, productName), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("Failed to list role project, sendRequest error: %v", err)
		return -1
	}
	var RoleProductModels RoleProductModels
	err = p.Deserialize([]byte(responseBody), &RoleProductModels.RoleProducts)
	if err != nil {
		log.Errorf("FindContributorRole Deserialize error: %v", err)
		return -1
	}
	for _, roleProduct := range RoleProductModels.RoleProducts {
		if roleProduct.Role.Name == setting.RoleContributor {
			return roleProduct.RoleID
		}
	}
	return -1
}

type RolePermission struct {
	RoleID         int64  `json:"roleId"`
	PermissionUUID string `json:"permissionUUID"`
}

type RolePermissionModels struct {
	RolePermissions []*RolePermission `json:"RolePermissions"`
}

//ListRolePermissions ...
func (p *PoetryClient) ListRolePermissions(roleID int64, roleType int, productName string, log *xlog.Logger) ([]*RolePermission, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/rolePermission?roleType=%d&roleId=%d&productName=%s", p.PoetryAPIServer, roleType, roleID, productName), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListRolePermissions SendRequest error: %v", err)
		return nil, fmt.Errorf("ListRolePermissions SendRequest error: %v", err)
	}
	var RolePermissionModels RolePermissionModels
	err = p.Deserialize([]byte(responseBody), &RolePermissionModels.RolePermissions)
	if err != nil {
		log.Errorf("ListRolePermissions Deserialize error: %v", err)
		return nil, fmt.Errorf("ListRolePermissions Deserialize error: %v", err)
	}
	return RolePermissionModels.RolePermissions, nil
}

type RoleEnv struct {
	RoleID      int    `json:"roleId"`
	EnvName     string `json:"envName"`
	ProductName string `json:"productName"`
}

type RoleEnvModels struct {
	RoleEnvs []*RoleEnv `json:"RoleEnvs"`
}

//ListRoleEnvs ...
func (p *PoetryClient) ListRoleEnvs(productName, envName string, roleID int64, log *xlog.Logger) ([]*RoleEnv, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/roleEnv?productName=%s&envName=%s&roleId=%d", p.PoetryAPIServer, productName, envName, roleID), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListRoleEnvs SendRequest error: %v", err)
		return nil, fmt.Errorf("ListRoleEnvs SendRequest error: %v", err)
	}
	var RoleEnvModels RoleEnvModels
	err = p.Deserialize([]byte(responseBody), &RoleEnvModels.RoleEnvs)
	if err != nil {
		log.Errorf("ListRoleEnvs Deserialize error: %v", err)
		return nil, fmt.Errorf("ListRoleEnvs Deserialize error: %v", err)
	}
	return RoleEnvModels.RoleEnvs, nil
}

func (p *PoetryClient) GetUserProject(userID int, log *xlog.Logger) (map[string][]int64, error) {
	productNameMap := make(map[string][]int64)
	userRoles, err := p.ListUserRoles(userID, ProjectType, log)
	if err != nil {
		log.Errorf("GetUserProject ListUserRoles error: %v", err)
		return productNameMap, fmt.Errorf("GetUserProject ListUserRoles error: %v", err)
	}
	for _, userRole := range userRoles {
		productNameMap[userRole.ProductName] = append(productNameMap[userRole.ProductName], userRole.RoleID)
	}
	return productNameMap, nil
}

func (p *PoetryClient) GetUserPermissionUUIDs(roleID int64, productName string, log *xlog.Logger) ([]string, error) {
	permissionUUIDs := make([]string, 0)
	rolePermissions, err := p.ListRolePermissions(roleID, ProjectType, productName, log)
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
func (p *PoetryClient) ListProductPermissionUsers(productName, permissionId string, log *xlog.Logger) ([]string, error) {
	responseBody, err := p.SendRequest(fmt.Sprintf("%s/directory/productUser?productName=%s&permissionId=%s", p.PoetryAPIServer, productName, permissionId), "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListProductPermissionUsers SendRequest error: %v", err)
		return nil, fmt.Errorf("ListProductPermissionUsers SendRequest error: %v", err)
	}
	resp := make([]string, 0)
	err = p.Deserialize([]byte(responseBody), &resp)
	if err != nil {
		log.Errorf("ListProductPermissionUsers Deserialize error: %v", err)
		return nil, fmt.Errorf("ListProductPermissionUsers Deserialize error: %v", err)
	}
	return resp, nil
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

func (p *PoetryClient) CreateContributorRole(product string, log *xlog.Logger) error {
	// 先创建角色
	header := p.GetRootTokenHeader()
	header.Set("content-type", "application/json")
	req := Role{
		IsDisabled:  false,
		Name:        setting.RoleContributor,
		ProductName: product,
		RoleType:    ProjectType,
		UpdateBy:    setting.SystemUser,
		UpdateAt:    time.Now().Unix(),
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		log.Errorf("CreateContributorRole failed, marshal json error: %v", err)
		return err
	}
	resp, err := p.Do("/directory/roles", "POST", bytes.NewBuffer(reqBody), header)
	if err != nil {
		log.Errorf("CreateContributorRole failed, call poetry API error: %v", err)
		return err
	}
	createdRole := new(Role)
	err = json.Unmarshal(resp, createdRole)
	if err != nil {
		log.Errorf("CreateContributorRole failed, cannot decode poetry response err: %v", err)
		return err
	}
	// 然后创建role product
	roleProductReq := RoleProduct{
		RoleID:      createdRole.ID,
		ProductName: product,
	}
	roleProductReqBody, err := json.Marshal(roleProductReq)
	if err != nil {
		log.Errorf("CreateContributorRole failed, marshal json error: %v", err)
		return err
	}
	_, err = p.Do("/directory/roleProduct", "POST", bytes.NewBuffer(roleProductReqBody), header)
	if err != nil {
		log.Errorf("CreateContributorRole failed, call poetry API create roleProduct error: %v", err)
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
		RoleID:         createdRole.ID,
		RoleType:       createdRole.RoleType,
		PermissionUUID: permissionList,
		ProductName:    product,
	}
	permissionReqBody, err := json.Marshal(permissionReq)
	if err != nil {
		log.Errorf("CreateContributorRole failed, marshal json error: %v", err)
		return err
	}
	_, err = p.Do("/directory/rolePermission", "POST", bytes.NewBuffer(permissionReqBody), header)
	if err != nil {
		log.Errorf("CreateContributorRole failed, call poetry API create rolePermission error: %v", err)
		return err
	}
	return nil
}

// 根据项目里面的角色名称获取对应的角色ID
func (p *PoetryClient) ListRoles(productName string, log *xlog.Logger) (*Role, error) {
	requestUrl := fmt.Sprintf("%s/directory/roles?name=%s&productName=%s&roleType=%d", p.PoetryAPIServer, AllUsersRoleName, productName, ProjectType)
	responseBody, err := p.SendRequest(requestUrl, "GET", "", p.GetRootTokenHeader())
	if err != nil {
		log.Errorf("ListRoles SendRequest error: %v", err)
		return nil, fmt.Errorf("ListRoles SendRequest error: %v", err)
	}
	resp := make([]*Role, 0)
	err = p.Deserialize([]byte(responseBody), &resp)
	if err != nil {
		log.Errorf("ListRoles Deserialize error: %v", err)
		return nil, fmt.Errorf("ListRoles Deserialize error: %v", err)
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("ListRoles Not Found")
	}
	return resp[0], nil
}

type UserEnv struct {
	ProductName    string `bson:"product_name"`
	EnvName        string `bson:"env_name"`
	UserID         int    `bson:"user_id"`
	PermissionUUID string `bson:"permission_uuid"`
}

func (p *PoetryClient) ListUserEnvPermission(productName string, userID int, log *xlog.Logger) ([]*UserEnv, error) {
	header := p.GetRootTokenHeader()
	api := fmt.Sprintf("/directory/userEnvPermission?userId=%d&productName=%s", userID, productName)
	responseBody, err := p.Do(api, "GET", nil, header)
	if err != nil {
		log.Errorf("ListUserEnvPermission SendRequest error: %v", err)
		return nil, err
	}

	resp := make([]*UserEnv, 0)
	err = p.Deserialize(responseBody, &resp)
	if err != nil {
		log.Errorf("ListUserEnvPermission Deserialize error: %v", err)
		return nil, err
	}

	return resp, nil
}

func (p *PoetryClient) DeleteUserEnvPermission(productName, username string, userID int, log *xlog.Logger) error {
	header := p.GetRootTokenHeader()
	api := fmt.Sprintf("/directory/userEnvPermission?userId=%d&productName=%s&envName=%s", userID, productName, strings.ToLower(username))
	_, err := p.Do(api, "DELETE", nil, header)
	if err != nil {
		log.Errorf("DeleteUserEnvPermission error: %v", err)
		return err
	}
	return nil
}
