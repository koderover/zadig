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

type ListStatsOption struct {
	ProductName string
	Event       string
	Limit       int
}

type StatsColl struct {
	*mongo.Collection

	coll string
}

func NewStatsColl() *StatsColl {
	name := models.Stats{}.TableName()
	return &StatsColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *StatsColl) GetCollectionName() string {
	return c.coll
}

func (c *StatsColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "req_id", Value: 1},
			bson.E{Key: "user", Value: 1},
			bson.E{Key: "event", Value: 1},
			bson.E{Key: "start_time", Value: 1},
			bson.E{Key: "end_time", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

// Create ...
func (c *StatsColl) Create(args *models.Stats) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil Stats")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

// List ...
func (c *StatsColl) List(option ListStatsOption) ([]*models.Stats, error) {
	var res []*models.Stats
	query := bson.M{}
	if option.Event != "" {
		query["event"] = option.Event
	}
	if option.Event != "" {
		query["ctx"] = bson.M{"ProductName": option.ProductName}
	}

	opts := options.Find()
	if option.Limit != 0 {
		opts.SetLimit(int64(option.Limit))
	}

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
