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
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ApplicationColl struct {
	*mongo.Collection
	coll string
}

func NewApplicationColl() *ApplicationColl {
	name := commonmodels.Application{}.TableName()
	return &ApplicationColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}
func (c *ApplicationColl) GetCollectionName() string { return c.coll }
func (c *ApplicationColl) EnsureIndex(ctx context.Context) error {
	idxes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "project", Value: 1}}},
		{Keys: bson.D{{Key: "language", Value: 1}}},
		{Keys: bson.D{{Key: "name", Value: 1}}},
		{Keys: bson.D{{Key: "create_time", Value: -1}}},
		{Keys: bson.D{{Key: "update_time", Value: -1}}},
		{Keys: bson.D{{Key: "repository.codehost_id", Value: 1}}},
	}
	_, err := c.Indexes().CreateMany(ctx, idxes)
	return err
}
func (c *ApplicationColl) Create(ctx context.Context, app *commonmodels.Application) (primitive.ObjectID, error) {
	if app == nil {
		return primitive.NilObjectID, errors.New("nil application")
	}
	now := time.Now().Unix()
	if app.CreateTime == 0 {
		app.CreateTime = now
	}
	app.UpdateTime = now
	res, err := c.InsertOne(ctx, app)
	if err != nil {
		return primitive.NilObjectID, err
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("unexpected inserted id type")
	}
	return oid, nil
}

func (c *ApplicationColl) GetByID(ctx context.Context, id string) (*commonmodels.Application, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	res := new(commonmodels.Application)
	err = c.FindOne(ctx, bson.M{"_id": oid}).Decode(res)
	return res, err
}

func (c *ApplicationColl) UpdateByID(ctx context.Context, id string, app *commonmodels.Application) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("nil application")
	}
	app.UpdateTime = time.Now().Unix()
	_, err = c.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": app})
	return err
}

func (c *ApplicationColl) DeleteByID(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = c.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

type ApplicationListOptions struct {
	Query          bson.M
	Sort           bson.D
	Page, PageSize int64
}

func (c *ApplicationColl) List(ctx context.Context, opt *ApplicationListOptions) ([]*commonmodels.Application, int64, error) {
	if opt == nil {
		opt = &ApplicationListOptions{}
	}
	if opt.Query == nil {
		opt.Query = bson.M{}
	}
	if opt.Page <= 0 {
		opt.Page = 1
	}
	if opt.PageSize <= 0 {
		opt.PageSize = 20
	}
	total, err := c.CountDocuments(ctx, opt.Query)
	if err != nil {
		return nil, 0, err
	}
	findOpts := options.Find().SetSkip((opt.Page - 1) * opt.PageSize).SetLimit(opt.PageSize)
	if len(opt.Sort) > 0 {
		findOpts.SetSort(opt.Sort)
	}
	cur, err := c.Find(ctx, opt.Query, findOpts)
	if err != nil {
		return nil, 0, err
	}
	var resp []*commonmodels.Application
	if err := cur.All(ctx, &resp); err != nil {
		return nil, 0, err
	}
	return resp, total, nil
}

type ApplicationFieldDefinitionColl struct {
	*mongo.Collection
	coll string
}

func NewApplicationFieldDefinitionColl() *ApplicationFieldDefinitionColl {
	name := commonmodels.ApplicationFieldDefinition{}.TableName()
	return &ApplicationFieldDefinitionColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}
func (c *ApplicationFieldDefinitionColl) GetCollectionName() string { return c.coll }
func (c *ApplicationFieldDefinitionColl) EnsureIndex(ctx context.Context) error {
	_, err := c.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)}})
	return err
}
func (c *ApplicationFieldDefinitionColl) Create(ctx context.Context, def *commonmodels.ApplicationFieldDefinition) (primitive.ObjectID, error) {
	if def == nil {
		return primitive.NilObjectID, errors.New("nil definition")
	}
	now := time.Now().Unix()
	if def.CreateTime == 0 {
		def.CreateTime = now
	}
	def.UpdateTime = now
	res, err := c.InsertOne(ctx, def)
	if err != nil {
		return primitive.NilObjectID, err
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("unexpected inserted id type")
	}
	return oid, nil
}
func (c *ApplicationFieldDefinitionColl) GetByKey(ctx context.Context, key string) (*commonmodels.ApplicationFieldDefinition, error) {
	res := new(commonmodels.ApplicationFieldDefinition)
	err := c.FindOne(ctx, bson.M{"key": key}).Decode(res)
	return res, err
}
func (c *ApplicationFieldDefinitionColl) List(ctx context.Context) ([]*commonmodels.ApplicationFieldDefinition, error) {
	cur, err := c.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var defs []*commonmodels.ApplicationFieldDefinition
	if err := cur.All(ctx, &defs); err != nil {
		return nil, err
	}
	return defs, nil
}
func (c *ApplicationFieldDefinitionColl) UpdateByKey(ctx context.Context, key string, def *commonmodels.ApplicationFieldDefinition) error {
	if def == nil {
		return errors.New("nil definition")
	}
	def.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(ctx, bson.M{"key": key}, bson.M{"$set": def})
	return err
}
func (c *ApplicationFieldDefinitionColl) DeleteByKey(ctx context.Context, key string) error {
	_, err := c.DeleteOne(ctx, bson.M{"key": key})
	return err
}
