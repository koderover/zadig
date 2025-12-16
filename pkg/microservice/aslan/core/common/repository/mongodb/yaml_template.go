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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type YamlTemplateColl struct {
	*mongo.Collection

	coll string
}

func NewYamlTemplateColl() *YamlTemplateColl {
	name := models.YamlTemplate{}.TableName()
	return &YamlTemplateColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *YamlTemplateColl) GetCollectionName() string {
	return c.coll
}

func (c *YamlTemplateColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *YamlTemplateColl) Create(obj *models.YamlTemplate) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *YamlTemplateColl) Update(idString string, obj *models.YamlTemplate) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *YamlTemplateColl) UpdateVariable(idString string, variable string, serviceVariableKVs []*commontypes.ServiceVariableKV) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{
		"variable_yaml":        variable,
		"service_variable_kvs": serviceVariableKVs,
	}}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *YamlTemplateColl) List(pageNum, pageSize int) ([]*models.YamlTemplate, int, error) {
	resp := make([]*models.YamlTemplate, 0)
	query := bson.M{}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}
	opt := options.Find()

	if pageNum > 0 && pageSize > 0 {
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

func (c *YamlTemplateColl) GetById(idstring string) (*models.YamlTemplate, error) {
	resp := new(models.YamlTemplate)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *YamlTemplateColl) GetByName(name string) (*models.YamlTemplate, error) {
	resp := new(models.YamlTemplate)
	query := bson.M{"name": name}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *YamlTemplateColl) DeleteByID(idstring string) error {
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
