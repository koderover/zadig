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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type TestStatOption struct {
	StartDate    int64
	EndDate      int64
	IsAsc        bool
	ProductNames []string
}

type TestStatColl struct {
	*mongo.Collection

	coll string
}

func NewTestStatColl() *TestStatColl {
	name := models.TestStat{}.TableName()
	return &TestStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *TestStatColl) GetCollectionName() string {
	return c.coll
}

func (c *TestStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "date", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *TestStatColl) Create(args *models.TestStat) error {
	if args == nil {
		return errors.New("nil testStat args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *TestStatColl) FindCount() (int, error) {
	cnt, err := c.Collection.CountDocuments(context.TODO(), bson.M{})
	return int(cnt), err
}

func (c *TestStatColl) ListTestStat(option *TestStatOption) (testStats []*models.TestStat, err error) {
	testStats = make([]*models.TestStat, 0)
	query := bson.M{}
	if len(option.ProductNames) > 0 {
		query["product_name"] = bson.M{"$in": option.ProductNames}
	}

	if option.StartDate > 0 {
		query["create_time"] = bson.M{"$gte": option.StartDate, "$lte": option.EndDate}
	}

	opt := &options.FindOptions{}
	if option.IsAsc {
		opt.SetSort(bson.D{{"create_time", 1}})
	} else {
		opt.SetSort(bson.D{{"create_time", -1}})
	}
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &testStats)
	if err != nil {
		return nil, err
	}
	return testStats, nil
}

func (c *TestStatColl) Update(args *models.TestStat) error {
	if args == nil {
		return errors.New("nil testStat args")
	}

	query := bson.M{"date": args.Date, "product_name": args.ProductName}
	_, err := c.UpdateOne(context.TODO(), query, args)
	return err
}
