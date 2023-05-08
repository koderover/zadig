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
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.17.0", "1.18.0", V1170ToV1180)
	upgradepath.RegisterHandler("1.18.0", "1.17.0", V1180ToV1170)
}

func V1170ToV1180() error {
	if err := migrateJiraAuthType(); err != nil {
		log.Errorf("migrateJiraAuthType err: %v", err)
	}

	if err := migrateProductAlias(); err != nil {
		log.Errorf("migrateProductAlias err: %v", err)
	}
	return nil
}

func V1180ToV1170() error {
	return nil
}

func migrateJiraAuthType() error {
	if jira, err := mongodb.NewProjectManagementColl().GetJira(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return errors.Wrap(err, "get jira")
	} else {
		if jira.JiraAuthType != "" {
			log.Warnf("migrateJiraAuthType: find jira auth type %s, skip", jira.JiraAuthType)
			return nil
		}
		jira.JiraAuthType = config.JiraBasicAuth
		if err := mongodb.NewProjectManagementColl().UpdateByID(jira.ID.Hex(), jira); err != nil {
			return errors.Wrap(err, "update")
		}
	}
	return nil
}

func migrateProductAlias() error {
	log.Info("update product alias field")
	mdb := mongodb.NewProductColl()

	products, err := mdb.List(&mongodb.ProductListOptions{})
	if err != nil {
		return err
	}
	if products == nil {
		return nil
	}

	var ms []mongo.WriteModel
	for _, product := range products {
		if product.EnvName != "" && product.Alias == "" {
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", product.ID}}).
					SetUpdate(bson.D{{"$set", bson.D{{"alias", product.EnvName}}}}),
			)
		}
	}
	if _, err = mdb.BulkWrite(context.TODO(), ms); err != nil {
		return err
	}
	return nil
}
