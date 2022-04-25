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

package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/microservice/picket/client/user"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
)

type DeleteUserResp struct {
	Message string `json:"message"`
}

func DeleteUser(userID string, header http.Header, qs url.Values, _ *zap.SugaredLogger) ([]byte, error) {
	_, err := user.New().DeleteUser(userID, header, qs)
	if err != nil {
		return []byte{}, err
	}
	return policy.New().DeleteRoleBindings(userID, header, qs)
}

func SearchUsers(header http.Header, qs url.Values, args *user.SearchArgs, log *zap.SugaredLogger) (*UsersResp, error) {
	users, err := user.New().SearchUsers(header, qs, args)
	if err != nil {
		log.Errorf("search users err :%s", err)
		return nil, err
	}
	tmpUids := []string{}
	for _, user := range users.Users {
		tmpUids = append(tmpUids, user.Uid)
	}

	systemMap, err := policy.New().SearchSystemRoleBindings(tmpUids)
	if err != nil {
		log.Errorf("search system user bindings err :%s", err)
		return nil, err
	}

	res := &UsersResp{
		Users:      make([]UserInfo, 0),
		TotalCount: users.TotalCount,
	}
	for _, user := range users.Users {
		userInfo := UserInfo{
			LastLoginTime: user.LastLoginTime,
			Uid:           user.Uid,
			Name:          user.Name,
			IdentityType:  user.IdentityType,
			Email:         user.Email,
			Phone:         user.Phone,
			Account:       user.Account,
			APIToken:      user.APIToken,
		}
		if rb, ok := systemMap[user.Uid]; ok {
			userInfo.SystemRoleBindings = rb
			for _, binding := range rb {
				if binding.Role == string(setting.SystemAdmin) {
					userInfo.Admin = true
					break
				}
			}
		}
		res.Users = append(res.Users, userInfo)
	}

	return res, nil
}

type UsersResp struct {
	Users      []UserInfo `json:"users"`
	TotalCount int64      `json:"totalCount"`
}

type UserInfo struct {
	LastLoginTime      int64                  `json:"lastLoginTime"`
	Uid                string                 `json:"uid"`
	Name               string                 `json:"name"`
	IdentityType       string                 `gorm:"default:'unknown'" json:"identity_type"`
	Email              string                 `json:"email"`
	Phone              string                 `json:"phone"`
	Account            string                 `json:"account"`
	APIToken           string                 `json:"token"`
	SystemRoleBindings []*service.RoleBinding `json:"system_role_bindings"`
	Admin              bool                   `json:"admin"`
}
