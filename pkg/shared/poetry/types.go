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

const (
	ProjectOwner = 3
)

type User interface {
	GetName() string
	GetEmail() string
	GetPhone() string
}

type UserInfo struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	Phone          string `json:"phone"`
	IsAdmin        bool   `json:"isAdmin"`
	IsSuperUser    bool   `json:"isSuperUser"`
	IsTeamLeader   bool   `json:"isTeamLeader"`
	OrganizationID int    `json:"organization_id"`
	Directory      string `json:"directory"`
	LastLoginAt    int64  `json:"lastLogin"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type ResponseMessage struct {
	ResultCode int    `json:"resultCode"`
	ErrorMsg   string `json:"errorMsg"`
}

type EnvRolePermission struct {
	ID             int64  `json:"id"`
	ProductName    string `json:"productName"`
	EnvName        string `json:"envName"`
	RoleID         int64  `json:"roleId"`
	PermissionUUID string `json:"permissionUUID"`
	RoleName       string `json:"roleName"`
}
