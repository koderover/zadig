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

type NotifyColl struct {
	*mongo.Collection

	coll string
}

func NewNotifyColl() *NotifyColl {
	name := models.Notify{}.TableName()
	return &NotifyColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *NotifyColl) GetCollectionName() string {
	return c.coll
}

func (c *NotifyColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *NotifyColl) Create(args *models.Notify) error {
	args.CreateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *NotifyColl) Read(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	change := bson.M{"$set": bson.M{
		"is_read": true,
	}}
	_, err = c.UpdateByID(context.TODO(), oid, change)

	return err
}

func (c *NotifyColl) Update(id string, args *models.Notify) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = c.UpdateByID(context.TODO(), oid, bson.M{"$set": args})

	return err
}

func (c *NotifyColl) List(receiver string) ([]*models.Notify, error) {
	var res []*models.Notify

	query := bson.M{"receiver": receiver}
	opts := options.Find().SetSort(bson.D{{"create_time", -1}}).SetLimit(100)
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *NotifyColl) DeleteByTime(expireTime int64) error {
	query := bson.M{"create_time": bson.M{"$lte": expireTime}}
	_, err := c.DeleteMany(context.TODO(), query)

	return err
}

func (c *NotifyColl) DeleteByID(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}
