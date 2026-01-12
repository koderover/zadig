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
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DashboardConfigColl struct {
	*mongo.Collection

	coll string
}

func NewDashboardConfigColl() *DashboardConfigColl {
	name := models.DashboardConfig{}.TableName()
	return &DashboardConfigColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DashboardConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *DashboardConfigColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "user_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *DashboardConfigColl) CreateOrUpdate(args *models.DashboardConfig) error {
	if args == nil {
		return errors.New("nil Install")
	}

	args.UpdateTime = time.Now().Unix()

	query := bson.M{"user_id": args.UserID, "user_name": args.UserName}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, args, opts)

	return err
}

func (c *DashboardConfigColl) GetByUser(userName, userID string) (*models.DashboardConfig, error) {
	query := bson.M{"user_id": userID, "user_name": userName}
	cfg := new(models.DashboardConfig)
	err := c.FindOne(context.TODO(), query).Decode(cfg)

	return cfg, err
}
