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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ListWorfklowQueueOption struct {
	WorkflowName string
	Status       config.Status
}

type WorkflowQueueColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowQueueColl() *WorkflowQueueColl {
	name := models.WorkflowQueue{}.TableName()
	return &WorkflowQueueColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkflowQueueColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowQueueColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "task_id", Value: 1},
			bson.E{Key: "workflow_name", Value: 1},
			bson.E{Key: "create_time", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *WorkflowQueueColl) List(opt *ListWorfklowQueueOption) ([]*models.WorkflowQueue, error) {
	query := bson.M{}
	if opt != nil {
		if opt.WorkflowName != "" {
			query["workflow_name"] = opt.WorkflowName
		}
		if opt.Status != "" {
			query["status"] = opt.Status
		}
	}

	var resp []*models.WorkflowQueue
	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{"create_time", 1}})
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *WorkflowQueueColl) Delete(args *models.WorkflowQueue) error {
	if args == nil {
		return errors.New("nil workflow queue")
	}

	query := bson.M{"task_id": args.TaskID, "workflow_name": args.WorkflowName, "create_time": args.CreateTime}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *WorkflowQueueColl) Create(args *models.WorkflowQueue) error {
	if args == nil {
		return errors.New("nil workflow queue")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *WorkflowQueueColl) Update(args *models.WorkflowQueue) error {
	if args == nil {
		return errors.New("nil workflow queue")
	}

	query := bson.M{"task_id": args.TaskID, "workflow_name": args.WorkflowName, "create_time": args.CreateTime}
	change := bson.M{"$set": bson.M{
		"status": args.Status,
		"stages": args.Stages,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
