/*
 * Copyright 2023 The KodeRover Authors.
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
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.19.0", "2.0.0", V1190ToV200)
	upgradepath.RegisterHandler("2.0.0", "1.19.0", V200ToV1190)
}

func V1190ToV200() error {
	log.Infof("-------- start migrate db instance project --------")
	if err := migrateDBIstanceProject(); err != nil {
		log.Infof("migrateDBIstanceProject err: %v", err)
		return err
	}

	return nil
}

func V200ToV1190() error {
	return nil
}

func migrateDBIstanceProject() error {
	var actions []*usermodels.Action
	err := repository.DB.Where("action = ?", "get_dbinstance_management").Find(&actions).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get action equal get_dbinstance_management, err: %v", err)
	}
	if len(actions) == 0 {
		actions = []*usermodels.Action{
			{Name: "查看", Action: "get_dbinstance_management", Resource: "DBInstanceManagement", Scope: 2},
			{Name: "新建", Action: "create_dbinstance_management", Resource: "DBInstanceManagement", Scope: 2},
			{Name: "编辑", Action: "edit_dbinstance_management", Resource: "DBInstanceManagement", Scope: 2},
			{Name: "删除", Action: "delete_dbinstance_management", Resource: "DBInstanceManagement", Scope: 2},
		}

		err = repository.DB.Create(actions).Error
		if err != nil {
			return fmt.Errorf("failed to create actions, err: %v", err)
		}
	}

	var dbInstances []*models.DBInstance
	query := bson.M{
		"projects": bson.M{"$exists": false},
	}
	cursor, err := mongodb.NewDBInstanceColl().Collection.Find(context.TODO(), query)
	if err != nil {
		return err
	}
	err = cursor.All(context.TODO(), &dbInstances)
	if err != nil {
		return err
	}
	for _, dbInstance := range dbInstances {
		dbInstance.Projects = append(dbInstance.Projects, setting.AllProjects)
		if err := mongodb.NewDBInstanceColl().Update(dbInstance.ID.Hex(), dbInstance); err != nil {
			return fmt.Errorf("failed to update db instance %s for migrateDBIstanceProject, err: %v", dbInstance.ID.Hex(), err)
		}
	}

	return nil
}
