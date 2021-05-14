/*
Copyright 2021 The KodeRover Authors.

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

package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type WebHookUserOption struct {
	Email    string
	UserName string
}

type WebHookUserColl struct {
	*mongo.Collection

	coll string
}

func NewWebHookUserColl() *WebHookUserColl {
	name := models.WebHookUser{}.TableName()
	return &WebHookUserColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WebHookUserColl) GetCollectionName() string {
	return c.coll
}

func (c *WebHookUserColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *WebHookUserColl) Create(args *models.WebHookUser) error {
	if args == nil || args.Email == "" {
		return errors.New("nil webHookUser args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *WebHookUserColl) Upsert(args *models.WebHookUser) error {
	if args == nil || args.Email == "" {
		return errors.New("nil webHookUser args")
	}

	query := bson.M{"email": args.Email}
	args.CreatedAt = time.Now().Unix()
	change := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *WebHookUserColl) UserExists(args *WebHookUserOption) (int, error) {
	query := bson.M{}
	if args.Email != "" {
		query["email"] = args.Email
	}

	if args.UserName != "" {
		query["user_name"] = args.UserName
	}

	count, err := c.CountDocuments(context.Background(), query)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (c *WebHookUserColl) FindWebHookUserCount() (int, error) {
	count, err := c.EstimatedDocumentCount(context.Background())
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (c *WebHookUserColl) List() ([]*models.WebHookUser, error) {
	webHookUsers := make([]*models.WebHookUser, 0)

	query := bson.M{"domain": bson.M{"$exists": true}}
	ctx := context.Background()
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &webHookUsers)
	if err != nil {
		return nil, err
	}

	return webHookUsers, nil
}
