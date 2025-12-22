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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ReleasePlanTemplateQueryOption struct {
	ID   string
	Name string
}

type ReleasePlanTemplateListOption struct {
	CreatedBy string
}

type ReleasePlanTemplateColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanTemplateColl() *ReleasePlanTemplateColl {
	name := models.ReleasePlanTemplate{}.TableName()
	return &ReleasePlanTemplateColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanTemplateColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanTemplateColl) EnsureIndex(ctx context.Context) error {
	index := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "template_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, index, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *ReleasePlanTemplateColl) Create(obj *models.ReleasePlanTemplate) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	obj.ID = primitive.NilObjectID
	obj.CreateTime = time.Now().Unix()
	obj.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *ReleasePlanTemplateColl) Update(obj *models.ReleasePlanTemplate) error {
	query := bson.M{"_id": obj.ID}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ReleasePlanTemplateColl) UpsertByName(obj *models.ReleasePlanTemplate) error {
	query := bson.M{"template_name": obj.TemplateName}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *ReleasePlanTemplateColl) Find(opt *ReleasePlanTemplateQueryOption) (*models.ReleasePlanTemplate, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}
	query := bson.M{}
	if len(opt.ID) > 0 {
		id, err := primitive.ObjectIDFromHex(opt.ID)
		if err != nil {
			return nil, err
		}
		query["_id"] = id
	}
	if len(opt.Name) > 0 {
		query["template_name"] = opt.Name
	}
	resp := new(models.ReleasePlanTemplate)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ReleasePlanTemplateColl) List(option *ReleasePlanTemplateListOption) ([]*models.ReleasePlanTemplate, error) {
	resp := make([]*models.ReleasePlanTemplate, 0)
	query := bson.M{}
	if option.CreatedBy != "" {
		query["create_by"] = option.CreatedBy
	}
	opt := options.Find().
		SetSort(bson.D{{"template_name", -1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ReleasePlanTemplateColl) DeleteByID(idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

type ListReleasePlanTemplateOption struct {
	Names []string
}

func (c *ReleasePlanTemplateColl) ListByCursor(opt *ListReleasePlanTemplateOption) (*mongo.Cursor, error) {
	query := bson.M{}

	if len(opt.Names) > 0 {
		query["template_name"] = bson.M{"$in": opt.Names}
	}

	return c.Collection.Find(context.TODO(), query)
}
