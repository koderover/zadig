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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type WorkflowV4JiraHookColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowV4JiraHookColl() *WorkflowV4JiraHookColl {
	name := models.WorkflowV4JiraHook{}.TableName()
	return &WorkflowV4JiraHookColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV4JiraHookColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV4JiraHookColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "name", Value: 1},
				bson.E{Key: "workflow_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "workflow_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *WorkflowV4JiraHookColl) Create(ctx *internalhandler.Context, obj *models.WorkflowV4JiraHook) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object")
	}

	if obj.WorkflowArg != nil {
		obj.WorkflowName = obj.WorkflowArg.Name
		obj.ProjectName = obj.WorkflowArg.Project
	}

	res, err := c.InsertOne(ctx, obj)
	if err != nil {
		return "", err
	}
	ID, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", errors.New("failed to get object id from create")
	}
	return ID.Hex(), err
}

func (c *WorkflowV4JiraHookColl) Exists(ctx *internalhandler.Context, workflowName, hookName string) (bool, error) {
	if err := c.Collection.FindOne(ctx, bson.M{"workflow_name": workflowName, "name": hookName}); err != nil {
		if err.Err() == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err.Err()
	}

	return true, nil
}

func (c *WorkflowV4JiraHookColl) List(ctx *internalhandler.Context, workflowName string) ([]*models.WorkflowV4JiraHook, error) {
	filter := bson.M{}
	if workflowName != "" {
		filter["workflow_name"] = workflowName
	}

	cursor, err := c.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var hooks []*models.WorkflowV4JiraHook
	if err := cursor.All(ctx, &hooks); err != nil {
		return nil, err
	}

	return hooks, nil
}

func (c *WorkflowV4JiraHookColl) Get(ctx *internalhandler.Context, workflowName, hookName string) (*models.WorkflowV4JiraHook, error) {
	var hook *models.WorkflowV4JiraHook
	if err := c.Collection.FindOne(ctx, bson.M{"workflow_name": workflowName, "name": hookName}).Decode(&hook); err != nil {
		return nil, err
	}

	return hook, nil
}

func (c *WorkflowV4JiraHookColl) Update(ctx *internalhandler.Context, id string, obj *models.WorkflowV4JiraHook) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = c.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": obj})
	if err != nil {
		return err
	}

	return nil
}

func (c *WorkflowV4JiraHookColl) Delete(ctx *internalhandler.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = c.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}

	return nil
}
