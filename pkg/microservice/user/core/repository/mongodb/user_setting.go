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
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type UserSettingColl struct {
	*mongo.Collection

	coll string
}

func NewUserSettingColl() *UserSettingColl {
	name := models.UserSetting{}.TableName()
	return &UserSettingColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *UserSettingColl) GetCollectionName() string {
	return c.coll
}

func (c *UserSettingColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "uid", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *UserSettingColl) UpsertUserSetting(args *models.UserSetting) error {
	if args == nil {
		return errors.New("nil UserSetting args")
	}
	query := bson.M{"uid": args.UID}
	change := bson.M{"$set": args}
	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *UserSettingColl) GetUserSettingByUid(uid string) (*models.UserSetting, error) {
	query := bson.M{"uid": uid}

	resp := &models.UserSetting{}

	err := c.FindOne(context.TODO(), query).Decode(resp)
	if err != nil && err != mongo.ErrNoDocuments {
		return resp, err
	}
	if err == mongo.ErrNoDocuments {
		return &models.UserSetting{}, nil
	}
	return resp, nil
}

func (c *UserSettingColl) DeleteUserSettingByUid(uid string) error {
	query := bson.M{"uid": uid}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
