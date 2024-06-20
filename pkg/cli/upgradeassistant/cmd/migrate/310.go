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
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func init() {
	upgradepath.RegisterHandler("3.0.0", "3.1.0", V300ToV310)
	upgradepath.RegisterHandler("3.1.0", "3.0.0", V310ToV300)
}

func V300ToV310() error {
	log.Infof("-------- start migrate infrastructure filed in testing, scanning and sacnning template module --------")
	if err := migrateTestingAndScaningInfraField(); err != nil {
		log.Infof("migrate infrastructure filed in testing, scanning and sacnning template module job err: %v", err)
		return err
	}

	return nil
}

func V310ToV300() error {
	return nil
}

func migrateTestingAndScaningInfraField() error {
	// change testing infrastructure field
	cursor, err := mongodb.NewTestingColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list testing cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var testing models.Testing
		if err := cursor.Decode(&testing); err != nil {
			return err
		}

		if testing.Infrastructure == "" {
			testing.Infrastructure = setting.JobK8sInfrastructure
			testing.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", testing.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", testing.Infrastructure},
							{"script_type", testing.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d testing", len(ms))
			if _, err := mongodb.NewTestingColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update testing for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d testing", len(ms))
		if _, err := mongodb.NewTestingColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update testing for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	// change scanning infrastructure field
	cursor, err = mongodb.NewScanningColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list scanning cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	ms = []mongo.WriteModel{}
	for cursor.Next(context.Background()) {
		var scanning models.Scanning
		if err := cursor.Decode(&scanning); err != nil {
			return err
		}

		if scanning.Infrastructure == "" {
			scanning.Infrastructure = setting.JobK8sInfrastructure
			scanning.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", scanning.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", scanning.Infrastructure},
							{"script_type", scanning.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d scanning", len(ms))
			if _, err := mongodb.NewScanningColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update sacnning for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d scanning", len(ms))
		if _, err := mongodb.NewScanningColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update scanning for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	// change scanning template infrastructure field
	cursor, err = mongodb.NewScanningTemplateColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list scanning template cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	ms = []mongo.WriteModel{}
	for cursor.Next(context.Background()) {
		var scanningTemplate models.ScanningTemplate
		if err := cursor.Decode(&scanningTemplate); err != nil {
			return err
		}

		if scanningTemplate.Infrastructure == "" {
			scanningTemplate.Infrastructure = setting.JobK8sInfrastructure
			scanningTemplate.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", scanningTemplate.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", scanningTemplate.Infrastructure},
							{"script_type", scanningTemplate.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d scanning template", len(ms))
			if _, err := mongodb.NewScanningTemplateColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update scanning template for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d scanning template", len(ms))
		if _, err := mongodb.NewScanningTemplateColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update scanning template for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	return nil
}
