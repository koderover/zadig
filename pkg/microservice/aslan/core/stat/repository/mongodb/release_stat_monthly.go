/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MonthlyReleaseStatColl struct {
	*mongo.Collection

	coll string
}

func NewMonthlyReleaseStatColl() *MonthlyReleaseStatColl {
	name := models.MonthlyReleaseStat{}.TableName()
	return &MonthlyReleaseStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *MonthlyReleaseStatColl) GetCollectionName() string {
	return c.coll
}

func (c *MonthlyReleaseStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "date", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *MonthlyReleaseStatColl) Upsert(args *models.MonthlyReleaseStat) error {
	if args == nil {
		return fmt.Errorf("upsert data cannot be empty for %s", "release_stat_monthly")
	}

	filter := bson.M{
		"date": args.Date,
	}

	update := bson.M{
		"$set": args,
	}

	updateOptions := options.Update().SetUpsert(true)

	_, err := c.UpdateOne(context.TODO(), filter, update, updateOptions)
	return err
}

func (c *MonthlyReleaseStatColl) List(startTime, endTime int64) ([]*models.MonthlyReleaseStat, error) {
	query := bson.M{}

	timeQuery := bson.M{}
	if startTime > 0 {
		timeQuery["$gte"] = startTime
	}
	if endTime > 0 {
		timeQuery["$lt"] = endTime
	}

	if startTime > 0 || endTime > 0 {
		query["create_time"] = timeQuery
	}

	cursor, err := c.Collection.Find(context.TODO(), query, options.Find())
	if err != nil {
		return nil, err
	}

	resp := make([]*models.MonthlyReleaseStat, 0)
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
