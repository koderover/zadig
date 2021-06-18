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

package permission

const (
	AnonymousUserID   = 0
	AnonymousUserName = "Anonymous"
)

type User struct {
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

var AnonymousUser = &User{ID: AnonymousUserID, Name: AnonymousUserName}
