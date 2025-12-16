/*
Copyright 2023 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type StatDashboardConfigColl struct {
	*mongo.Collection

	coll string
}

func NewStatDashboardConfigColl() *StatDashboardConfigColl {
	name := models.StatDashboardConfig{}.TableName()
	return &StatDashboardConfigColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *StatDashboardConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *StatDashboardConfigColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "item_key", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *StatDashboardConfigColl) Create(ctx context.Context, args *models.StatDashboardConfig) error {
	if args == nil {
		return errors.New("statistics dashboard configuration is nil")
	}

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *StatDashboardConfigColl) BulkCreate(ctx context.Context, args []*models.StatDashboardConfig) error {
	if args == nil {
		return errors.New("statistics dashboard configuration list is nil")
	}

	inputs := make([]interface{}, 0)
	for _, arg := range args {
		inputs = append(inputs, arg)
	}

	_, err := c.InsertMany(ctx, inputs)
	return err
}

func (c *StatDashboardConfigColl) List() ([]*models.StatDashboardConfig, error) {
	resp := make([]*models.StatDashboardConfig, 0)

	query := bson.M{}
	cursor, err := c.Collection.Find(context.TODO(), query, options.Find())
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *StatDashboardConfigColl) Update(ctx context.Context, key string, args *models.StatDashboardConfig) error {
	query := bson.M{"item_key": key}
	change := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *StatDashboardConfigColl) Delete(ctx context.Context, key string) error {
	query := bson.M{"item_key": key}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

type ConfigOption struct {
	Type    string `bson:"type"`
	ItemKey string `bson:"item_key"`
}

func (c *StatDashboardConfigColl) FindByOptions(opts *ConfigOption) (*models.StatDashboardConfig, error) {
	if opts == nil {
		return nil, errors.New("statistics dashboard configuration is nil")
	}
	query := bson.M{}
	if opts.Type != "" {
		query["type"] = opts.Type
	}
	if opts.ItemKey != "" {
		query["item_key"] = opts.ItemKey
	}

	resp := &models.StatDashboardConfig{}
	err := c.FindOne(context.Background(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
