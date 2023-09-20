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

	"github.com/koderover/zadig/pkg/setting"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/types"
)

type DeleteUserResp struct {
	Message string `json:"message"`
}

func DeleteUser(userID string, header http.Header, qs url.Values, _ *zap.SugaredLogger) ([]byte, error) {
	return user.New().DeleteUser(userID, header, qs)
}

func SearchUsers(header http.Header, qs url.Values, args *user.SearchArgs, log *zap.SugaredLogger) (*types.UsersResp, error) {
	users, err := user.New().SearchUsers(header, qs, args)
	if err != nil {
		log.Errorf("search users err :%s", err)
		return nil, err
	}

	res := &types.UsersResp{
		Users:      make([]*types.UserInfo, 0),
		TotalCount: users.TotalCount,
	}

	for _, uInfo := range users.Users {
		userInfo := &types.UserInfo{
			LastLoginTime: uInfo.LastLoginTime,
			Uid:           uInfo.Uid,
			Name:          uInfo.Name,
			IdentityType:  uInfo.IdentityType,
			Email:         uInfo.Email,
			Phone:         uInfo.Phone,
			Account:       uInfo.Account,
			APIToken:      uInfo.APIToken,
		}
		roles, err := user.New().ListRoles("*", uInfo.Uid)
		if err != nil {
			log.Errorf("failed to get user role info for user: %s[%s], error: %s", uInfo.Name, uInfo.Account, err)
			return nil, err
		}
		rolebindings := make([]*types.RoleBinding, 0)
		for _, role := range roles {
			rolebindings = append(rolebindings, &types.RoleBinding{
				UID:  uInfo.Uid,
				Role: role.Name,
			})
			if role.Name == string(setting.SystemAdmin) {
				userInfo.Admin = true
			}
		}
		userInfo.SystemRoleBindings = rolebindings

		res.Users = append(res.Users, userInfo)
	}

	return res, nil
}
