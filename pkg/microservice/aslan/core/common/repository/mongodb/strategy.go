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

package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

const (
	fieldTarget = "target"
)

type StrategyColl struct {
	*mongo.Collection

	coll string
}

func NewStrategyColl() *StrategyColl {
	name := models.CapacityStrategy{}.TableName()
	return &StrategyColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *StrategyColl) GetCollectionName() string {
	return c.coll
}

func (c *StrategyColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *StrategyColl) Upsert(strategy *models.CapacityStrategy) error {
	query := bson.M{fieldTarget: strategy.Target}
	change := bson.M{"$set": strategy}

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

// GetByTarget returns either one selected CapacityStrategy or error
func (c *StrategyColl) GetByTarget(target models.CapacityTarget) (*models.CapacityStrategy, error) {
	query := bson.M{fieldTarget: target}
	result := &models.CapacityStrategy{}

	err := c.FindOne(context.TODO(), query).Decode(result)
	return result, err
}
