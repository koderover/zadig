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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type SubscriptionColl struct {
	*mongo.Collection

	coll string
}

func NewSubscriptionColl() *SubscriptionColl {
	name := models.Subscription{}.TableName()
	return &SubscriptionColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *SubscriptionColl) GetCollectionName() string {
	return c.coll
}

func (c *SubscriptionColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "subscriber", Value: 1},
			bson.E{Key: "type", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *SubscriptionColl) Upsert(args *models.Subscription) error {
	args.CreateTime = time.Now().Unix()
	query := bson.M{"subscriber": args.Subscriber, "type": args.Type}
	update := bson.M{"$set": args}
	_, err := c.UpdateOne(context.TODO(), query, update, options.Update().SetUpsert(true))

	return err
}

func (c *SubscriptionColl) Update(subscriber string, notifyType int, args *models.Subscription) error {
	query := bson.M{"subscriber": subscriber, "type": notifyType}
	change := bson.M{"$set": bson.M{
		"pipelinestatus": args.PipelineStatus,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *SubscriptionColl) Delete(subscriber string, notifyType int) error {
	query := bson.M{"subscriber": subscriber, "type": notifyType}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

func (c *SubscriptionColl) List(subscriber string) ([]*models.Subscription, error) {
	var res []*models.Subscription

	query := bson.M{"subscriber": subscriber}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)

	return res, err
}
