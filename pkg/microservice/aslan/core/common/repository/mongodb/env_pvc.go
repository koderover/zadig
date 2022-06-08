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

type PvcColl struct {
	*mongo.Collection

	coll string
}

func NewPvcColl() *PvcColl {
	name := models.EnvPvc{}.TableName()
	return &PvcColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *PvcColl) GetCollectionName() string {
	return c.coll
}

func (c *PvcColl) EnsureIndex(ctx context.Context) error {
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

func (c *PvcColl) Create(args *models.EnvPvc, isCreateTime bool) error {
	if isCreateTime {
		args.CreateTime = time.Now().Unix()
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *PvcColl) List(opt *ListEnvCfgOption) ([]*models.EnvPvc, error) {
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

	var resp []*models.EnvPvc
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

func (c *PvcColl) Update(id string, services []string) error {
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

func (c *PvcColl) Find(opt *FindEnvCfgOption) (*models.EnvPvc, error) {
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

	rs := &models.EnvPvc{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		return nil, err
	}
	return rs, err
}
