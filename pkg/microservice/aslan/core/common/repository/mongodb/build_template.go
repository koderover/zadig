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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type BuildTemplateQueryOption struct {
	ID   string
	Name string
}

type BuildTemplateColl struct {
	*mongo.Collection
	coll string
}

func NewBuildTemplateColl() *BuildTemplateColl {
	name := models.BuildTemplate{}.TableName()
	return &BuildTemplateColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *BuildTemplateColl) GetCollectionName() string {
	return c.coll
}

func (c *BuildTemplateColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *BuildTemplateColl) Create(obj *models.BuildTemplate) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	obj.ID = primitive.NilObjectID
	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *BuildTemplateColl) Update(idStr string, obj *models.BuildTemplate) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	obj.ID = id
	query := bson.M{"_id": id}
	change := bson.M{"$set": obj}
	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *BuildTemplateColl) Find(opt *BuildTemplateQueryOption) (*models.BuildTemplate, error) {
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
		query["name"] = opt.Name
	}
	resp := new(models.BuildTemplate)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *BuildTemplateColl) List(pageNum, pageSize int) ([]*models.BuildTemplate, int, error) {
	resp := make([]*models.BuildTemplate, 0)
	query := bson.M{}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}

	opt := options.Find()
	if pageNum != 0 && pageSize != 0 {
		opt.SetSkip(int64((pageNum - 1) * pageSize)).SetLimit(int64(pageSize))
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, int(count), nil
}

func (c *BuildTemplateColl) DeleteByName(name string) error {
	query := bson.M{"name": name}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *BuildTemplateColl) DeleteByID(idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

type ListBuildTemplateOption struct {
	Infrastructure string
}

func (c *BuildTemplateColl) ListByCursor(opts *ListBuildTemplateOption) (*mongo.Cursor, error) {
	query := bson.M{}

	return c.Collection.Find(context.Background(), query)
}
