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

type LabelBindingColl struct {
	*mongo.Collection

	coll string
}

func NewLabelBindingColl() *LabelBindingColl {
	name := models.LabelBinding{}.TableName()
	return &LabelBindingColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LabelBindingColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelBindingColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "resource_id", Value: 1},
			bson.E{Key: "label_id", Value: 1},
			bson.E{Key: "resource_type", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *LabelBindingColl) Create(args *models.LabelBinding) error {
	args.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type LabelBindingCollFindOpt struct {
	LabelID      string
	LabelIDs     []string
	ResourceID   string
	ResourceType string
}

func (c *LabelBindingColl) FindByOpt(opt *LabelBindingCollFindOpt) (*models.LabelBinding, error) {
	res := &models.LabelBinding{}
	query := bson.M{}
	if opt.LabelID != "" {
		query["label_id"] = opt.LabelID
	}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *LabelBindingColl) ListByOpt(opt *LabelBindingCollFindOpt) ([]*models.LabelBinding, error) {
	var ret []*models.LabelBinding
	query := bson.M{}
	if opt.LabelID != "" {
		query["label_id"] = opt.LabelID
	}
	if len(opt.LabelIDs) != 0 {
		query["label_id"] = bson.M{"$in": opt.LabelIDs}
	}
	if opt.ResourceID != "" {
		query["resource_id"] = opt.ResourceID
	}
	if opt.ResourceType != "" {
		query["resource_type"] = opt.ResourceType
	}
	ctx := context.Background()
	opts := options.Find()
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func (c *LabelBindingColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) DeleteMany(ids []string) error {
	query := bson.M{}
	var oids []primitive.ObjectID
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		oids = append(oids, oid)
	}
	query["_id"] = bson.M{"$in": oids}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
