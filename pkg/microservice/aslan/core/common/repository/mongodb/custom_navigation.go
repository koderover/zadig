/*
Copyright 2025 The KodeRover Authors.

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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CustomNavigationColl struct {
	*mongo.Collection

	coll string
}

func NewCustomNavigationColl() *CustomNavigationColl {
	name := models.CustomNavigation{}.TableName()
	return &CustomNavigationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *CustomNavigationColl) GetCollectionName() string { return c.coll }

func (c *CustomNavigationColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *CustomNavigationColl) Get() (*models.CustomNavigation, error) {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	query := bson.M{"_id": id}
	res := new(models.CustomNavigation)
	err := c.Collection.FindOne(context.Background(), query).Decode(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *CustomNavigationColl) CreateOrUpdate(nav *models.CustomNavigation) error {
	id, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	nav.UpdateTime = time.Now().Unix()
	if nav.CreateTime == 0 {
		nav.CreateTime = nav.UpdateTime
	}
	if nav.ID.IsZero() {
		nav.ID = id
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": nav}
	_, err := c.UpdateOne(context.Background(), query, change, options.Update().SetUpsert(true))
	return err
}
