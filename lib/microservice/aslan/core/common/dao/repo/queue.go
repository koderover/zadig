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

package repo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ListQueueOption struct {
	PipelineName   string
	Status         config.Status
	MergeRequestID string
}

type QueueColl struct {
	*mongo.Collection

	coll string
}

func NewQueueColl() *QueueColl {
	name := models.Queue{}.TableName()
	return &QueueColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *QueueColl) GetCollectionName() string {
	return c.coll
}

func (c *QueueColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "task_id", Value: 1},
			bson.E{Key: "pipeline_name", Value: 1},
			bson.E{Key: "create_time", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *QueueColl) List(opt *ListQueueOption) ([]*models.Queue, error) {
	query := bson.M{}
	if opt != nil {
		if opt.PipelineName != "" {
			query["pipeline_name"] = opt.PipelineName
		}
		if opt.Status != "" {
			query["status"] = opt.Status
		}
	}

	var resp []*models.Queue
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

func (c *QueueColl) Delete(args *models.Queue) error {
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	query := bson.M{"task_id": args.TaskID, "pipeline_name": args.PipelineName, "type": args.Type, "create_time": args.CreateTime}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *QueueColl) Create(args *models.Queue) error {
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *QueueColl) Update(args *models.Queue) error {
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	query := bson.M{"task_id": args.TaskID, "pipeline_name": args.PipelineName, "create_time": args.CreateTime}
	change := bson.M{"$set": bson.M{
		"status":     args.Status,
		"start_time": args.StartTime,
		"end_time":   args.EndTime,
		"sub_tasks":  args.SubTasks,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *QueueColl) UpdateAgent(taskID int64, pipelineName string, createTime int64, agentID string) error {
	query := bson.M{"task_id": taskID, "pipeline_name": pipelineName, "create_time": createTime, "agent_id": ""}
	change := bson.M{"$set": bson.M{
		"agent_id": agentID,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
