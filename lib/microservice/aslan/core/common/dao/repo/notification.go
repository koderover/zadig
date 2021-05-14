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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type NotificationColl struct {
	*mongo.Collection

	coll string
}

func NewNotificationColl() *NotificationColl {
	name := models.Notification{}.TableName()
	return &NotificationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *NotificationColl) GetCollectionName() string {
	return c.coll
}

func (c *NotificationColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *NotificationColl) Find(id string) (*models.Notification, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := &models.Notification{}
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *NotificationColl) Create(args *models.Notification) error {
	args.Created = time.Now().Unix()

	res, err := c.InsertOne(context.TODO(), args)
	if err != nil || res == nil {
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		args.ID = oid
	}

	return nil
}

func (c *NotificationColl) Upsert(args *models.Notification) error {
	query := bson.M{"_id": args.ID}
	args.Created = time.Now().Unix()
	update := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, update, options.Update().SetUpsert(true))

	return err
}
