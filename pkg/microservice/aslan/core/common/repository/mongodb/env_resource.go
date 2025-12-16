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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type EnvResourceColl struct {
	*mongo.Collection

	coll string
}

func NewEnvResourceColl() *EnvResourceColl {
	name := models.EnvResource{}.TableName()
	return &EnvResourceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvResourceColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvResourceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "create_time", Value: 1},
			bson.E{Key: "env_name", Value: 1},
			bson.E{Key: "type", Value: 1},
			bson.E{Key: "product_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *EnvResourceColl) Create(args *models.EnvResource) error {
	if args.CreateTime == 0 {
		args.CreateTime = time.Now().Unix()
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type QueryEnvResourceOption struct {
	Id             string
	CreateTime     string
	IsSort         bool
	ProductName    string
	Namespace      string
	EnvName        string
	Name           string
	Type           string
	IgnoreNotFound bool
	AutoSync       bool
	Active         bool
}

type EnvResourceBaseData struct {
	ProductName string `bson:"product_name"`
	EnvName     string `bson:"env_name"`
	Name        string `bson:"name"`
	Type        string `bson:"type"`
}

type EnvResourceAggregateData struct {
	ID         EnvResourceBaseData `bson:"_id"`
	CreateTime int64               `bson:"create_time"`
}

func (c *EnvResourceColl) List(opt *QueryEnvResourceOption) ([]*models.EnvResource, error) {
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
	if len(opt.Type) > 0 {
		query["type"] = opt.Type
	}

	var resp []*models.EnvResource
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

func (c *EnvResourceColl) Find(opt *QueryEnvResourceOption) (*models.EnvResource, error) {
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
	if len(opt.Type) > 0 {
		query["type"] = opt.Type
	}
	if opt.Active {
		query["deleted_at"] = 0
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

	rs := &models.EnvResource{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		if err == mongo.ErrNoDocuments && opt.IgnoreNotFound {
			return nil, nil
		}
		return nil, err
	}
	return rs, err
}

func (c *EnvResourceColl) ListLatestResource(opt *QueryEnvResourceOption) ([]*EnvResourceAggregateData, error) {
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
	if len(opt.Type) > 0 {
		query["type"] = opt.Type
	}

	var pipeline []bson.M
	if len(query) > 0 {
		pipeline = append(pipeline, bson.M{"$match": query})
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"create_time": -1}},
		bson.M{
			"$group": bson.M{
				"_id": bson.M{
					"product_name": "$product_name",
					"env_name":     "$env_name",
					"name":         "$name",
					"type":         "$type",
				},
				"create_time": bson.M{"$first": "$create_time"},
				"deleted_at":  bson.M{"$first": "$deleted_at"},
				"auto_sync":   bson.M{"$first": "$auto_sync"},
			},
		},
	)

	pipeline = append(pipeline, bson.M{
		"$match": bson.M{"deleted_at": 0},
	})

	if opt.AutoSync {
		pipeline = append(pipeline, bson.M{
			"$match": bson.M{"auto_sync": true},
		})
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	result := make([]*EnvResourceAggregateData, 0)
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	return result, err
}

func (c *EnvResourceColl) Delete(oid primitive.ObjectID) error {
	query := bson.M{}
	query["_id"] = oid
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
