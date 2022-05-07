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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SecretColl struct {
	*mongo.Collection

	coll string
}

func NewSecretColl() *SecretColl {
	name := models.EnvSecret{}.TableName()
	return &SecretColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *SecretColl) GetCollectionName() string {
	return c.coll
}

func (c *SecretColl) EnsureIndex(ctx context.Context) error {
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

func (c *SecretColl) Create(args *models.EnvSecret, isCreateTime bool) error {
	if isCreateTime {
		args.CreateTime = time.Now().Unix()
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *SecretColl) List(opt *ListEnvCfgOption) ([]*models.EnvSecret, error) {
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

	var resp []*models.EnvSecret
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

func (c *SecretColl) Update(id string, services []string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"services": services,
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *SecretColl) Find(opt *FindEnvCfgOption) (*models.EnvSecret, error) {
	if opt == nil {
		return nil, errors.New("FindEnvCfgOption cannot be nil")
	}
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
	if len(opt.Id) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.Id)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	opts := options.FindOne()
	if len(opt.CreateTime) > 0 {
		query["create_time"] = opt.CreateTime
	} else {
		opts.SetSort(bson.D{{"create_time", -1}})
	}

	rs := &models.EnvSecret{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		return nil, err
	}
	return rs, err
}
