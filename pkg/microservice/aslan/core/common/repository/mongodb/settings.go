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

package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/config"
	aslanConfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type SystemSettingColl struct {
	*mongo.Collection

	coll string
}

func NewSystemSettingColl() *SystemSettingColl {
	name := models.SystemSetting{}.TableName()
	return &SystemSettingColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *SystemSettingColl) GetCollectionName() string {
	return c.coll
}

func (c *SystemSettingColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *SystemSettingColl) Get() (*models.SystemSetting, error) {
	query := bson.M{}
	resp := &models.SystemSetting{}

	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *SystemSettingColl) UpdateDefaultLoginSetting(defaultLogin string) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	change := bson.M{"$set": bson.M{
		"default_login": defaultLogin,
	}}
	query := bson.M{"_id": id}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *SystemSettingColl) UpdateConcurrencySetting(workflowConcurrency, buildConcurrency int64) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	change := bson.M{"$set": bson.M{
		"workflow_concurrency": workflowConcurrency,
		"build_concurrency":    buildConcurrency,
	}}
	query := bson.M{"_id": id}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *SystemSettingColl) UpdateSecuritySetting(tokenExpirationTime int64) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	change := bson.M{"$set": bson.M{
		"security.token_expiration_time": tokenExpirationTime,
	}}
	query := bson.M{"_id": id}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *SystemSettingColl) UpdatePrivacySetting(improvementPlan bool) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	change := bson.M{"$set": bson.M{
		"privacy.improvement_plan": improvementPlan,
	}}
	query := bson.M{"_id": id}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *SystemSettingColl) InitSystemSettings() error {
	_, err := c.Get()
	// if we didn't find anything
	if err != nil {
		return c.CreateOrUpdate(setting.LocalClusterID, &models.SystemSetting{
			WorkflowConcurrency: 2,
			BuildConcurrency:    5,
			DefaultLogin:        setting.DefaultLoginLocal,
			Theme: &models.Theme{
				ThemeType: aslanConfig.CUSTOME_THEME,
				CustomTheme: &models.CustomTheme{
					BorderGray:               "#d2d7dc",
					FontGray:                 "#888888",
					FontLightGray:            "#a0a0a0",
					ThemeColor:               "#0066ff",
					ThemeBorderColor:         "#66bbff",
					ThemeBackgroundColor:     "#eeeeff",
					ThemeLightColor:          "#66bbff",
					BackgroundColor:          "#e5e5e5",
					GlobalBackgroundColor:    "#ffffff",
					Success:                  "#57d344",
					Danger:                   "#f93940",
					Warning:                  "#ff9000",
					Info:                     "#96989b",
					Primary:                  "#0066ff",
					WarningLight:             "#cdb62c",
					NotRunning:               "#303133",
					PrimaryColor:             "#000",
					SecondaryColor:           "#707275",
					SidebarBg:                "#f5f7fa",
					SidebarActiveColor:       "#e6f6ff",
					ProjectItemIconColor:     "#0066ff",
					ProjectNameColor:         "#121212",
					TableCellBackgroundColor: "#ffffff",
					LinkColor:                "#0066ff",
				},
			},
			Privacy:  &models.PrivacySettings{ImprovementPlan: true},
			Security: &models.SecuritySettings{TokenExpirationTime: 24},
		})
	}
	return nil
}

func (c *SystemSettingColl) CreateOrUpdate(id string, args *models.SystemSetting) error {
	var objectID primitive.ObjectID
	if id != "" {
		objectID, _ = primitive.ObjectIDFromHex(id)
	} else {
		objectID = primitive.NewObjectID()
	}

	args.UpdateTime = time.Now().Unix()

	query := bson.M{"_id": objectID}
	change := bson.M{"$set": args}
	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *SystemSettingColl) UpdateTheme(theme *models.Theme) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	query := bson.M{"_id": id}

	var change bson.M
	if theme.ThemeType != aslanConfig.CUSTOME_THEME {
		change = bson.M{"$set": bson.M{"theme.theme_type": theme.ThemeType}}
	} else {
		change = bson.M{"$set": bson.M{"theme": theme}}
	}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
