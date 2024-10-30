/*
 * Copyright 2024 The KodeRover Authors.
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

package migrate

import (
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	sprintservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/sprint_management/service"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
)

func init() {
	upgradepath.RegisterHandler("3.1.0", "3.2.0", V310ToV320)
	upgradepath.RegisterHandler("3.2.0", "3.1.0", V320ToV310)
}

func V310ToV320() error {
	ctx := handler.NewBackgroupContext()
	ctx.Logger.Infof("-------- start init existed project's sprint template --------")
	if err := initProjectSprintTemplate(ctx); err != nil {
		ctx.Logger.Errorf("failed to init project sprint template, error: %s", err)
		return err
	}
	if err := addGetSprintActionForReadOnlyRole(ctx); err != nil {
		ctx.Logger.Errorf("failed to add get_sprint action for read-only role, error: %s", err)
		return err
	}

	return nil
}

func V320ToV310() error {
	return nil
}

func initProjectSprintTemplate(ctx *handler.Context) error {
	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		err = fmt.Errorf("failed to list project list, error: %s", err)
		ctx.Logger.Error(err)
		return err
	}

	for _, project := range projects {
		sprintservice.InitSprintTemplate(ctx, project.ProjectName)
	}

	return nil
}

func addGetSprintActionForReadOnlyRole(ctx *handler.Context) error {
	action := &usermodels.Action{}
	err := repository.DB.Where("action = ? AND resource = ?", "get_sprint", "SprintManagement").First(&action).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get SprintManagement/get_sprint action, err: %v", err)
	}

	roles := []*usermodels.NewRole{}
	err = repository.DB.Where("name = ?", "read-only").Find(&roles).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		err = fmt.Errorf("failed to get read-only roles, err: %v", err)
		ctx.Logger.Error(err)
		return err
	}
	if len(roles) == 0 {
		ctx.Logger.Infof("role read-only not found")
		return nil
	}

	roleBindings := []*usermodels.RoleActionBinding{}
	for _, role := range roles {
		roleBindings = append(roleBindings, &usermodels.RoleActionBinding{
			RoleID:   role.ID,
			ActionID: action.ID,
		})
	}

	err = repository.DB.Create(roleBindings).Error
	if err != nil {
		return fmt.Errorf("failed to create roleBingdings, err: %v", err)
	}

	return nil
}
