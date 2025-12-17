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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type WorkflowTasKRevertColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowTaskRevertColl() *WorkflowTasKRevertColl {
	name := models.WorkflowTaskRevert{}.TableName()
	return &WorkflowTasKRevertColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkflowTasKRevertColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowTasKRevertColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "task_id", Value: 1},
				bson.E{Key: "job_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"create_time": 1},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *WorkflowTasKRevertColl) Create(obj *models.WorkflowTaskRevert) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object")
	}

	res, err := c.InsertOne(context.TODO(), obj)
	if err != nil {
		return "", err
	}
	ID, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", fmt.Errorf("failed to get object id from create")
	}
	return ID.Hex(), err
}

func (c *WorkflowTasKRevertColl) UpateStatusByID(revertID string, status config.Status) error {
	id, err := primitive.ObjectIDFromHex(revertID)
	if err != nil {
		return fmt.Errorf("invalid object id %s", revertID)
	}
	query := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err = c.Collection.UpdateOne(context.TODO(), query, update)
	if err != nil {
		return err
	}

	return nil
}

func (c *WorkflowTasKRevertColl) UpateStatus(workflowName string, taskID int64, jobName string, status config.Status) error {
	query := bson.M{}
	if workflowName != "" {
		query["workflow_name"] = workflowName
	}
	if taskID != 0 {
		query["task_id"] = taskID
	}
	if jobName != "" {
		query["job_name"] = jobName
	}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, update)
	if err != nil {
		return err
	}

	return nil
}

type ListWorkflowRevertOption struct {
	TaskID       int64
	WorkflowName string
	JobName      string
}

func (c *WorkflowTasKRevertColl) List(opt *ListWorkflowRevertOption) ([]*models.WorkflowTaskRevert, error) {
	resp := make([]*models.WorkflowTaskRevert, 0)

	query := bson.M{}
	if opt.WorkflowName != "" {
		query["workflow_name"] = opt.WorkflowName
	}
	if opt.TaskID != 0 {
		query["task_id"] = opt.TaskID
	}
	if opt.JobName != "" {
		query["job_name"] = opt.JobName
	}

	findOption := options.Find().SetSort(bson.M{"create_time": -1})
	cursor, err := c.Collection.Find(context.TODO(), query, findOption)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
