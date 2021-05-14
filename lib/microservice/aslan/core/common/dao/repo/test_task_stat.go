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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type TestTaskStatOption struct {
	Name string
}

type TestTaskStatColl struct {
	*mongo.Collection

	coll string
}

func NewTestTaskStatColl() *TestTaskStatColl {
	name := models.TestTaskStat{}.TableName()
	return &TestTaskStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *TestTaskStatColl) GetCollectionName() string {
	return c.coll
}

func (c *TestTaskStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *TestTaskStatColl) Delete(name string) error {
	query := bson.M{}
	if name != "" {
		query["name"] = name
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *TestTaskStatColl) ListTestTask() ([]*models.TestTaskStat, error) {
	testTaskStats := make([]*models.TestTaskStat, 0)
	query := bson.M{}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &testTaskStats)
	if err != nil {
		return nil, err
	}
	return testTaskStats, nil
}

func (c *TestTaskStatColl) FindTestTaskStat(option *TestTaskStatOption) (*models.TestTaskStat, error) {
	query := bson.M{"name": option.Name}
	testTaskStat := new(models.TestTaskStat)

	err := c.FindOne(context.TODO(), query).Decode(testTaskStat)
	return testTaskStat, err
}

func (c *TestTaskStatColl) Create(args *models.TestTaskStat) error {
	if args == nil {
		return errors.New("nil testTaskStat args")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *TestTaskStatColl) Update(args *models.TestTaskStat) error {
	if args == nil {
		return errors.New("nil testTaskStat args")
	}

	query := bson.M{"name": args.Name}
	change := bson.M{"$set": args}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}
