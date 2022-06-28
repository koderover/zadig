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

// Note this file should be deleted in future

package mongodb

import (
	"context"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type IngressColl struct {
	*mongo.Collection

	coll string
}

func NewIngressColl() *IngressColl {
	name := models.EnvIngress{}.TableName()
	return &IngressColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *IngressColl) GetCollectionName() string {
	return c.coll
}

func (c *IngressColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "create_time", Value: 1},
			bson.E{Key: "env_name", Value: 1},
			bson.E{Key: "product_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *IngressColl) Create(args *models.EnvIngress, isCreateTime bool) error {
	if isCreateTime {
		args.CreateTime = time.Now().Unix()
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *IngressColl) List(opt *ListEnvCfgOption) ([]*models.EnvIngress, error) {
	query := bson.M{}
	if len(opt.ProductName) > 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.Namespace) > 0 {
		query["namespace"] = opt.Namespace
	}
	if len(opt.EnvName) > 0 {
		query["env_name"] = opt.EnvName
	}
	if len(opt.Name) > 0 {
		query["name"] = opt.Name
	}

	var resp []*models.EnvIngress
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"create_time", -1}})
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
