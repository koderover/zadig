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

	"github.com/koderover/zadig/v2/pkg/setting"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type WorkflowTemplateQueryOption struct {
	ID   string
	Name string
}

type WorkflowTemplateListOption struct {
	CreatedBy      string
	Category       string
	ExcludeBuildIn bool
}

type WorkflowV4TemplateColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowV4TemplateColl() *WorkflowV4TemplateColl {
	name := models.WorkflowV4Template{}.TableName()
	return &WorkflowV4TemplateColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV4TemplateColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV4TemplateColl) EnsureIndex(ctx context.Context) error {
	index := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "template_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, index, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *WorkflowV4TemplateColl) Create(obj *models.WorkflowV4Template) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	obj.ID = primitive.NilObjectID
	obj.CreateTime = time.Now().Unix()
	obj.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *WorkflowV4TemplateColl) Update(obj *models.WorkflowV4Template) error {
	query := bson.M{"_id": obj.ID}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *WorkflowV4TemplateColl) UpsertByName(obj *models.WorkflowV4Template) error {
	query := bson.M{"template_name": obj.TemplateName}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *WorkflowV4TemplateColl) DeleteInternalByName(name string) error {
	query := bson.M{"template_name": name, "created_by": "system"}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *WorkflowV4TemplateColl) Find(opt *WorkflowTemplateQueryOption) (*models.WorkflowV4Template, error) {
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
	resp := new(models.WorkflowV4Template)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowV4TemplateColl) List(option *WorkflowTemplateListOption) ([]*models.WorkflowV4Template, error) {
	resp := make([]*models.WorkflowV4Template, 0)
	query := bson.M{}
	if option.CreatedBy != "" {
		query["create_by"] = option.CreatedBy
	}
	if option.Category != "" {
		query["category"] = option.Category
	}
	if option.ExcludeBuildIn {
		query["build_in"] = false
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

func (c *WorkflowV4TemplateColl) DeleteByID(idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

type ListWorkflowV4TemplateOption struct {
	Names    []string
	Category setting.WorkflowCategory
}

func (c *WorkflowV4TemplateColl) ListByCursor(opt *ListWorkflowV4TemplateOption) (*mongo.Cursor, error) {
	query := bson.M{}

	if len(opt.Names) > 0 {
		query["template_name"] = bson.M{"$in": opt.Names}
	}
	if opt.Category != "" {
		query["category"] = opt.Category
	}

	return c.Collection.Find(context.TODO(), query)
}
