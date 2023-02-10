/*
Copyright 2022 The KodeRover Authors.

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

package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	policymongo "github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.16.0", "1.17.0", V1160ToV1170)
	upgradepath.RegisterHandler("1.16.0", "1.15.0", V1170ToV1160)
}

func V1160ToV1170() error {
	if err := updateRolesForTesting(); err != nil {
		log.Errorf("updateRolesForTesting err:%s", err)
		return err
	}
	if err := migrateJiraInfo(); err != nil {
		log.Errorf("migrateJiraInfo err: %v", err)
		return err
	}
	return nil
}

func V1170ToV1160() error {
	return nil
}

func updateRolesForTesting() error {
	roles, err := policymongo.NewRoleColl().List()
	if err != nil {
		return fmt.Errorf("list roles error: %s", err)
	}
	var mRoles []mongo.WriteModel
	for _, role := range roles {
		if role.Namespace != "*" {
			continue
		}
		for _, rule := range role.Rules {
			if len(rule.Resources) == 0 {
				continue
			}
			if rule.Resources[0] == "TestCenter" {
				newVerbs := []string{}
				for _, verb := range rule.Verbs {
					if verb == "get_test" {
						newVerbs = append(newVerbs, verb)
					}
				}
				rule.Verbs = newVerbs
			}
		}
		mRoles = append(mRoles,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"namespace", role.Namespace}, {"name", role.Name}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"rules", role.Rules},
					}},
				}),
		)
		if len(mRoles) >= 50 {
			log.Infof("update %d roles", len(mRoles))
			if _, err := policymongo.NewRoleColl().BulkWrite(context.TODO(), mRoles); err != nil {
				return fmt.Errorf("udpate workflowV4s error: %s", err)
			}
			mRoles = []mongo.WriteModel{}
		}
	}
	if len(mRoles) > 0 {
		if _, err := policymongo.NewRoleColl().BulkWrite(context.TODO(), mRoles); err != nil {
			return fmt.Errorf("udpate roles stat error: %s", err)
		}
	}
	return nil
}

func migrateJiraInfo() error {
	list, err := mongodb.NewProjectManagementColl().List()
	if err == nil {
		for _, management := range list {
			if management.Type == setting.PMJira {
				log.Warnf("migrateJiraInfo: find V1170 jira info, skip migrate")
				return nil
			}
		}
	}

	type JiraOld struct {
		Host        string `bson:"host"`
		User        string `bson:"user"`
		AccessToken string `bson:"access_token"`
	}
	coll := mongotool.Database(config.MongoDatabase()).Collection("jira")
	info := &JiraOld{}
	err = coll.FindOne(context.Background(), bson.M{}).Decode(info)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Info("migrateJiraInfo: not found old jira info, done")
			return nil
		}
		return err
	}

	log.Infof("migrateJiraInfo: find info, host: %s user: %s", info.Host, info.User)
	return mongodb.NewProjectManagementColl().Create(&commonmodels.ProjectManagement{
		Type:      setting.PMJira,
		JiraHost:  info.Host,
		JiraUser:  info.User,
		JiraToken: info.AccessToken,
	})
}
