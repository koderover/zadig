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
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	models2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type OperationLogArgs struct {
	Username     string `json:"username"`
	ProductName  string `json:"product_name"`
	ExactProduct string `json:"exact_product"`
	Function     string `json:"function"`
	Status       int    `json:"status"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	Scene        string `json:"scene"`
	TargetID     string `json:"target_id"`
	Detail       string `json:"detail"`
}

type OperationLogColl struct {
	*mongo.Collection

	coll string
}

func NewOperationLogColl() *OperationLogColl {
	name := models2.OperationLog{}.TableName()
	return &OperationLogColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *OperationLogColl) GetCollectionName() string {
	return c.coll
}

func (c *OperationLogColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "product_name", Value: 1},
				{Key: "scene", Value: 1},
				{Key: "targets", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "function", Value: 1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}}, // Sorting index
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *OperationLogColl) Insert(args *models2.OperationLog) error {
	if args == nil {
		return errors.New("nil operation_log args")
	}

	res, err := c.InsertOne(context.TODO(), args)
	if err != nil || res == nil {
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		args.ID = oid
	}

	return nil
}

func (c *OperationLogColl) Update(id string, status int) error {
	if id == "" {
		return errors.New("nil operation_log args")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	change := bson.M{"$set": bson.M{
		"status": status,
	}}
	_, err = c.UpdateByID(context.TODO(), oid, change, options.Update().SetUpsert(true))

	return err
}

func (c *OperationLogColl) Find(args *OperationLogArgs) ([]*models2.OperationLog, int, error) {
	var res []*models2.OperationLog
	query := bson.M{}
	if args.ProductName != "" {
		query["product_name"] = bson.M{"$regex": args.ProductName}
	}
	if args.ExactProduct != "" {
		query["product_name"] = args.ExactProduct
	}
	if args.Username != "" {
		query["username"] = bson.M{"$regex": args.Username}
	}
	if args.Function != "" {
		query["function"] = bson.M{"$regex": args.Function}
	}
	if args.Status != 0 {
		query["status"] = args.Status
	}
	if args.Scene != "" {
		query["scene"] = args.Scene
	}
	if args.TargetID != "" {
		query["targets"] = bson.M{"$elemMatch": bson.M{"$eq": args.TargetID}}
	}
	if args.Detail != "" {
		query["name"] = bson.M{"$regex": args.Detail}
	}

	opts := options.Find()
	opts.SetSort(bson.D{{"created_at", -1}})
	if args.Page > 0 && args.PerPage > 0 {
		opts.SetSkip(int64(args.PerPage * (args.Page - 1))).SetLimit(int64(args.PerPage))
	}
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, 0, err
	}

	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}

	return res, int(count), err
}
