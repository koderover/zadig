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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type LabelColl struct {
	*mongo.Collection

	coll string
}

func NewLabelColl() *LabelColl {
	name := models.Label{}.TableName()
	return &LabelColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LabelColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "key", Value: 1},
			bson.E{Key: "value", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *LabelColl) Create(args *models.Label) error {
	args.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *LabelColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *LabelColl) Find(id string) (*models.Label, error) {
	res := new(models.Label)
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	opts := options.FindOne()
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query, opts).Decode(res)
	return res, err
}

type ListLabelOpt struct {
	Key    string
	Values []string
	Type   string
}

func (c *LabelColl) List(opt *ListLabelOpt) ([]*models.Label, error) {
	var res []*models.Label

	ctx := context.Background()

	query := bson.M{}
	if opt.Key != "" {
		query["key"] = opt.Key
	}
	if len(opt.Values) != 0 {
		query["value"] = bson.M{"$in": opt.Values}
	}
	if opt.Type != "" {
		query["type"] = opt.Type
	}
	opts := options.Find()
	opts.SetSort(bson.D{{"key", 1}})

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
