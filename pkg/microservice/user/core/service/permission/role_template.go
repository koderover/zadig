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
	"errors"
	"fmt"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ListRoleTemplates(log *zap.SugaredLogger) ([]*types.RoleTemplate, error) {
	roles, err := orm.ListRoleTemplates(repository.DB)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("failed to list role templates, error: %s", err)
		return nil, fmt.Errorf("failed to list role templates, error: %s", err)
	}

	resp := make([]*types.RoleTemplate, 0)
	for _, role := range roles {
		resp = append(resp, &types.RoleTemplate{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
		})
	}

	return resp, nil
}

func CreateRoleTemplate(req *CreateRoleReq, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	roleTemplate := &models.RoleTemplate{
		Name:        req.Name,
		Description: req.Desc,
	}

	err := orm.CreateRoleTemplate(roleTemplate, tx)
	if err != nil {
		log.Errorf("failed to create role, error: %s", err)
		tx.Rollback()
		return fmt.Errorf("failed to create role, error: %s", err)
	}

	actionIDList := make([]uint, 0)
	actionList := make([]string, 0)
	for _, action := range req.Actions {
		// if the action is not in the action cache, get one.
		if _, ok := ActionMap[action]; !ok {
			act, err := orm.GetActionByVerb(action, repository.DB)
			if err != nil {
				log.Errorf("failed to find verb: %s in request, action might not exist.", action)
				tx.Rollback()
				return fmt.Errorf("failed to find verb: %s in request, action might not exist", action)
			}
			ActionMap[action] = act.ID
		}
		actionIDList = append(actionIDList, ActionMap[action])
		actionList = append(actionList, action)
	}

	err = orm.BulkCreateRoleTemplateActionBindings(roleTemplate.ID, actionIDList, tx)
	if err != nil {
		log.Errorf("failed to create action binding for role template: %s, the error is: %s", roleTemplate.Name, err)
		tx.Rollback()
		return fmt.Errorf("failed to create action binding for role template: %s, the error is: %s", roleTemplate.Name, err)
	}

	tx.Commit()
	return nil
}

func UpdateRoleTemplate(req *CreateRoleReq, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	// Doing a tricky thing here: removing the whole role-action binding, then re-adding them.
	roleTemplateInfo, err := orm.GetRoleTemplate(req.Name, repository.DB)
	if err != nil {
		log.Errorf("failed to find role: [%s], error: %s", req.Name, err)
		tx.Rollback()
		return fmt.Errorf("failed to find role: [%s], error: %s", req.Name, err)
	}

	err = orm.DeleteRoleTemplateActionBindingByRole(roleTemplateInfo.ID, tx)
	if err != nil {
		log.Errorf("failed to delete role-action binding for role: %s, error: %s", roleTemplateInfo.Name, err)
		tx.Rollback()
		return fmt.Errorf("update role-action binding failed, error: %s", err)
	}

	actionIDList := make([]uint, 0)
	actionList := make([]string, 0)
	for _, action := range req.Actions {
		// if the action is not in the action cache, get one.
		if _, ok := ActionMap[action]; !ok {
			act, err := orm.GetActionByVerb(action, repository.DB)
			if err != nil {
				log.Errorf("failed to find verb: %s in request, action might not exist.", action)
				tx.Rollback()
				return fmt.Errorf("failed to find verb: %s in request, action might not exist", action)
			}
			ActionMap[action] = act.ID
		}
		actionIDList = append(actionIDList, ActionMap[action])
		actionList = append(actionList, action)
	}

	err = orm.BulkCreateRoleTemplateActionBindings(roleTemplateInfo.ID, actionIDList, tx)
	if err != nil {
		log.Errorf("failed to create action binding for role: %s , the error is: %s", roleTemplateInfo.Name, err)
		tx.Rollback()
		return fmt.Errorf("failed to create action binding for role: %s , the error is: %s", roleTemplateInfo.Name, err)
	}

	// so the only field capable of changing is the description....
	err = orm.UpdateRoleTemplateInfo(roleTemplateInfo.ID, &models.RoleTemplate{
		Description: req.Desc,
	}, tx)

	tx.Commit()

	return nil
}

func GetRoleTemplate(name string, log *zap.SugaredLogger) (*types.DetailedRoleTemplate, error) {
	role, err := orm.GetRoleTemplate(name, repository.DB)
	if err != nil {
		log.Errorf("failed to find role: %s, error: %s", name, err)
		return nil, fmt.Errorf("failed to find role: %s, error: %s", name, err)
	}

	actionList, err := orm.ListActionByRoleTemplate(role.ID, repository.DB)
	if err != nil {
		log.Errorf("failed to find action for role: %s under namespace: %s, error: %s", name, err)
		return nil, fmt.Errorf("failed to find action for role: %s, error: %s", name, err)
	}

	actionMap := make(map[string]sets.String)

	for _, action := range actionList {
		if _, ok := actionMap[action.Resource]; !ok {
			actionMap[action.Resource] = sets.NewString()
		}

		actionMap[action.Resource].Insert(action.Action)
	}

	resourceActionList := make([]*types.ResourceAction, 0)
	for resource, actionSet := range actionMap {
		resourceActionList = append(resourceActionList, &types.ResourceAction{
			Resource: resource,
			Verbs:    actionSet.List(),
		})
	}

	resp := &types.DetailedRoleTemplate{
		ID:              role.ID,
		Name:            role.Name,
		Namespace:       "*",
		Description:     role.Description,
		ResourceActions: resourceActionList,
	}

	return resp, nil
}

func DeleteRoleTemplate(name string, log *zap.SugaredLogger) error {
	err := orm.DeleteRoleTemplateByName(name, repository.DB)
	if err != nil {
		log.Errorf("failed to delete role template: %s, error: %s", name, err)
		return fmt.Errorf("failed to delete role: %s, error: %s", name, err)
	}
	return nil
}
