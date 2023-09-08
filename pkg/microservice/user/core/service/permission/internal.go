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

	"github.com/koderover/zadig/pkg/types"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
)

func InitializeProjectAuthorization(namespace string, isPublic bool, admins []string, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()
	// First, create default roles
	projectAdminRole := &models.NewRole{
		Name:        "project-admin",
		Description: "",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}
	readOnlyRole := &models.NewRole{
		Name:        "read-only",
		Description: "",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}
	readProjectOnlyRole := &models.NewRole{
		Name:        "read-project-only",
		Description: "",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}

	err := orm.BulkCreateRole([]*models.NewRole{projectAdminRole, readOnlyRole, readProjectOnlyRole}, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
		return fmt.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
	}

	// then, create role bindings if the project is public
	if isPublic {
		group, err := orm.GetUserGroupByName(types.AllUserGroupName, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to find all-user group, error: %s", err)
			return fmt.Errorf("failed to find all-user group, error: %s", err)
		}

		err = orm.CreateGroupRoleBinding(&models.GroupRoleBinding{
			GroupID: group.GroupID,
			RoleID:  readOnlyRole.ID,
		}, tx)

		if err != nil {
			tx.Rollback()
			log.Errorf("failed to bind read-only role to all-users, error: %s", err)
			return fmt.Errorf("failed to bind read-only role to all-users, error: %s", err)
		}
	}

	// finally create role bindings
	err = orm.BulkCreateRoleBindingForRole(projectAdminRole.ID, admins, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to bind project-admin role to given user list, error: %s", err)
		return fmt.Errorf("failed to bind project-admin role to given user list, error: %s", err)
	}

	tx.Commit()
	return nil
}
