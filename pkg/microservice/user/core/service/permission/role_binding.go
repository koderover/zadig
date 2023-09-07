/*
Copyright 2023 The KodeRover Authors.

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

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
)

type RoleBindingResp struct {
	BindingType string            `json:"binding_type"`
	UserInfo    *BindingUserInfo  `json:"user_info,omitempty"`
	GroupInfo   *BindingGroupInfo `json:"group_info,omitempty"`
	Roles       []string          `json:"roles"`
}

type BindingUserInfo struct {
	IdentityType string `json:"identity_type"`
	UID          string `json:"uid"`
	Account      string `json:"account"`
	Username     string `json:"username"`
}

type BindingGroupInfo struct {
	GID  uint   `json:"group_id"`
	Name string `json:"name"`
}

type Identity struct {
	IdentityType string `json:"identity_type"`
	UID          string `json:"uid,omitempty"`
	GID          uint   `json:"gid,omitempty"`
}

func ListRoleBindings(ns, uid, gid string, log *zap.SugaredLogger) ([]*RoleBindingResp, error) {
	resp := make([]*RoleBindingResp, 0)

	userInfoMap := make(map[string]*BindingUserInfo)

	// first we deal with the user-role bindings
	userRoleMap := make(map[string]sets.String)
	if uid != "" {
		userInfo, err := orm.GetUserByUid(uid, repository.DB)
		if err != nil {
			log.Errorf("failed to get user info for uid: %s, error:%s", uid, err)
			return nil, fmt.Errorf("failed to get user info for uid: %s, error:%s", uid, err)
		}
		userInfoMap[uid] = &BindingUserInfo{
			IdentityType: userInfo.IdentityType,
			UID:          userInfo.UID,
			Account:      userInfo.Account,
			Username:     userInfo.Name,
		}
		roles, err := orm.ListRoleByUIDAndNamespace(uid, ns, repository.DB)
		if err != nil {
			log.Errorf("failed to list roles by uid: %s, namespace: %s, error is: %s", uid, ns, err)
			return nil, fmt.Errorf("failed to list roles by uid: %s, namespace: %s, error is: %s", uid, ns, err)
		}
		userRoleMap[uid] = sets.NewString()
		for _, role := range roles {
			userRoleMap[uid].Insert(role.Name)
		}
	} else {
		roleBindings, err := orm.ListRoleBindingByNamespace(ns, repository.DB)
		if err != nil {
			log.Errorf("failed to list role bindings in namespace: %s, error is: %s", ns, err)
			return nil, fmt.Errorf("failed to list role bindings in namespace: %s, error is: %s", ns, err)
		}
		for _, roleBinding := range roleBindings {
			if _, ok := userRoleMap[roleBinding.User.UID]; !ok {
				userRoleMap[roleBinding.User.UID] = sets.NewString()
			}

			if _, ok := userInfoMap[roleBinding.User.UID]; !ok {
				userInfoMap[roleBinding.User.UID] = &BindingUserInfo{
					IdentityType: roleBinding.User.IdentityType,
					UID:          roleBinding.User.UID,
					Account:      roleBinding.User.Account,
					Username:     roleBinding.User.Name,
				}
			}
			userRoleMap[roleBinding.User.UID].Insert(roleBinding.Role.Name)
		}
	}

	for userID, roleSets := range userRoleMap {
		resp = append(resp, &RoleBindingResp{
			BindingType: "user",
			UserInfo:    userInfoMap[userID],
			Roles:       roleSets.List(),
		})
	}

	// TODO: Add User Group logics

	return resp, nil
}

func CreateRoleBindings(role, ns string, identityList []*Identity, log *zap.SugaredLogger) error {
	roleInfo, err := orm.GetRole(role, ns, repository.DB)
	if err != nil || roleInfo.ID == 0 {
		log.Errorf("failed to find role: %s in namespace: %s, error: %s", role, ns, err)
		return fmt.Errorf("failed to find role: %s in namespace: %s, error: %s", role, ns, err)
	}

	// first split identities between user and groups
	userIDList := make([]string, 0)
	groupIDList := make([]uint, 0)
	for _, identity := range identityList {
		switch identity.IdentityType {
		case "user":
			log.Infof("adding user: %s into user list for role: %s in namespace: %s", identity.UID, role, ns)
			userIDList = append(userIDList, identity.UID)
		case "group":
			groupIDList = append(groupIDList, identity.GID)
		default:
			log.Errorf("invalid identity type detected: %s", identity.IdentityType)
			return fmt.Errorf("invalid identity type detected: %s", identity.IdentityType)
		}
	}

	tx := repository.DB.Begin()

	// create role bindings for users first
	err = orm.BulkCreateRoleBindingForRole(roleInfo.ID, userIDList, tx)
	if err != nil {
		log.Errorf("failed to create role binding for role: %s, error: %s", role, err)
		tx.Rollback()
		return fmt.Errorf("failed to create role binding for role: %s, error: %s", role, err)
	}

	// TODO: Add user group logics

	tx.Commit()
	return nil
}

func UpdateRoleBindingForUser(uid, namespace string, roles []string, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	roleIDList := make([]uint, 0)

	roleList, err := orm.ListRoleByRoleNamesAndNamespace(roles, namespace, repository.DB)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to find roles in the given role list, error: %s", err)
		return fmt.Errorf("update role binding failed, error: %s", err)
	}

	for _, role := range roleList {
		roleIDList = append(roleIDList, role.ID)
	}

	err = orm.DeleteRoleBindingByUID(uid, namespace, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to delete role bindings for user: %s under namespace: %s, error: %s", uid, namespace, err)
		return fmt.Errorf("update role binding failed, error: %s", err)
	}

	err = orm.BulkCreateRoleBindingForUser(uid, roleIDList, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to create new role bindings for user: %s under namespace %s, error: %s", uid, namespace, err)
		return fmt.Errorf("failed to create new role bindings for user: %s under namespace %s, error: %s", uid, namespace, err)
	}

	tx.Commit()

	return nil
}

func DeleteRoleBindingForUser(uid, namespace string, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	err := orm.DeleteRoleBindingByUID(uid, namespace, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to delete role bindings for user: %s under namespace: %s, error: %s", uid, namespace, err)
		return fmt.Errorf("delete role binding failed, error: %s", err)
	}

	tx.Commit()

	return nil
}
