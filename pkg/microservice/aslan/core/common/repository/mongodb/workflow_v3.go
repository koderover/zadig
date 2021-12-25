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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type WorkflowV3Coll struct {
	*mongo.Collection

	coll string
}

type ListWorkflowV3Option struct {
	ProjectName string
}

func NewWorkflowV3Coll() *WorkflowV3Coll {
	name := models.WorkflowV3{}.TableName()
	return &WorkflowV3Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV3Coll) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV3Coll) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"project_name": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *WorkflowV3Coll) Create(obj *models.WorkflowV3) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object")
	}

	res, err := c.InsertOne(context.TODO(), obj)
	if err != nil {
		return "", err
	}
	ID, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", errors.New("failed to get object id from create")
	}
	return ID.Hex(), err
}

func (c *WorkflowV3Coll) List(opt *ListWorkflowV3Option, pageNum, pageSize int64) ([]*models.WorkflowV3, int64, error) {
	resp := make([]*models.WorkflowV3, 0)
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}
	var findOption *options.FindOptions
	if pageNum == 0 && pageSize == 0 {
		findOption = options.Find()
	} else {
		findOption = options.Find().
			SetSkip((pageNum - 1) * pageSize).
			SetLimit(pageSize)
	}

	cursor, err := c.Collection.Find(context.TODO(), query, findOption)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, count, nil
}

func (c *WorkflowV3Coll) Find(name string) (*models.WorkflowV3, error) {
	resp := new(models.WorkflowV3)
	query := bson.M{"name": name}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowV3Coll) GetByID(idstring string) (*models.WorkflowV3, error) {
	resp := new(models.WorkflowV3)
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

func (c *WorkflowV3Coll) Update(idString string, obj *models.WorkflowV3) error {
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

func (c *WorkflowV3Coll) DeleteByID(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
