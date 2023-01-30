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
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ListWorkflowTaskV4Option struct {
	WorkflowName    string
	ProjectName     string
	ProjectNames    []string
	WorkflowNames   []string
	CreateTime      int64
	BeforeCreatTime bool
	Limit           int
	Skip            int
	IsSort          bool
}

type WorkflowTaskv4Coll struct {
	*mongo.Collection

	coll string
}

func NewworkflowTaskv4Coll() *WorkflowTaskv4Coll {
	name := models.WorkflowTask{}.TableName()
	return &WorkflowTaskv4Coll{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkflowTaskv4Coll) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowTaskv4Coll) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "task_id", Value: 1},
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
				bson.E{Key: "status", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"create_time": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "is_archived", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *WorkflowTaskv4Coll) Create(obj *models.WorkflowTask) (string, error) {
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

func (c *WorkflowTaskv4Coll) List(opt *ListWorkflowTaskV4Option) ([]*models.WorkflowTask, int64, error) {
	resp := make([]*models.WorkflowTask, 0)
	query := bson.M{}
	if opt.WorkflowName != "" {
		query["workflow_name"] = opt.WorkflowName
	}
	if opt.WorkflowNames != nil {
		query["workflow_name"] = bson.M{"$in": opt.WorkflowNames}
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	query["is_archived"] = false
	query["is_deleted"] = false
	if opt.CreateTime > 0 {
		comparison := "$gte"
		if opt.BeforeCreatTime {
			comparison = "$lte"
		}
		query["create_time"] = bson.M{comparison: opt.CreateTime}
	}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}

	findOption := options.Find()
	if opt.Limit > 0 {
		findOption.SetSort(bson.D{{"create_time", -1}})
		findOption.SetSkip(int64(opt.Skip))
		findOption.SetLimit(int64(opt.Limit))
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

func (c *WorkflowTaskv4Coll) GetLatest(workflowName string) (*models.WorkflowTask, error) {
	resp := new(models.WorkflowTask)
	query := bson.M{}
	query["workflow_name"] = workflowName

	findOption := options.FindOne()
	findOption.SetSort(bson.D{{"create_time", -1}})

	err := c.FindOne(context.TODO(), query, findOption).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowTaskv4Coll) FindTodoTasksByWorkflowName(workflowName string) ([]*models.WorkflowTask, error) {
	ret := make([]*models.WorkflowTask, 0)
	query := bson.M{"status": bson.M{"$in": []string{"waiting", "queued", "created", "running", "blocked"}}}
	query["workflow_name"] = workflowName
	query["is_deleted"] = false
	query["is_archived"] = false

	opt := options.Find()
	opt.SetSort(bson.D{{"create_time", 1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &ret)

	return ret, err
}

func (c *WorkflowTaskv4Coll) InCompletedTasks() ([]*models.WorkflowTask, error) {
	ret := make([]*models.WorkflowTask, 0)
	query := bson.M{"status": bson.M{"$in": []string{"created", "running"}}}
	query["is_deleted"] = false

	opt := options.Find()
	opt.SetSort(bson.D{{"create_time", 1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *WorkflowTaskv4Coll) Find(workflowName string, taskID int64) (*models.WorkflowTask, error) {
	resp := new(models.WorkflowTask)
	query := bson.M{"workflow_name": workflowName, "task_id": taskID}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowTaskv4Coll) GetByID(idstring string) (*models.WorkflowTask, error) {
	resp := new(models.WorkflowTask)
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

func (c *WorkflowTaskv4Coll) Update(idString string, obj *models.WorkflowTask) error {
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

func (c *WorkflowTaskv4Coll) DeleteByWorkflowName(workflowName string) error {
	query := bson.M{"workflow_name": workflowName}
	change := bson.M{"$set": bson.M{
		"is_deleted":  true,
		"is_archived": true,
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *WorkflowTaskv4Coll) ArchiveHistoryWorkflowTask(workflowName string, remain, remainDays int) error {
	if remain == 0 && remainDays == 0 {
		return nil
	}
	query := bson.M{"workflow_name": workflowName, "is_deleted": false}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return err
	}
	if remain > 0 {
		query["task_id"] = bson.M{"$lt": int(count) - remain + 1}
	}
	if remainDays > 0 {
		query["create_time"] = bson.M{"$lt": time.Now().AddDate(0, 0, -remainDays).Unix()}
	}
	change := bson.M{"$set": bson.M{
		"is_archived": true,
	}}
	_, err = c.UpdateMany(context.TODO(), query, change)

	return err
}

func (c *WorkflowTaskv4Coll) ListByCursor(opt *ListWorkflowTaskV4Option) (*mongo.Cursor, error) {
	query := bson.M{}
	if opt.WorkflowName != "" {
		query["workflow_name"] = opt.WorkflowName
	}
	if opt.WorkflowNames != nil {
		query["workflow_name"] = bson.M{"$in": opt.WorkflowNames}
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	if len(opt.ProjectNames) > 0 {
		query["product_name"] = bson.M{"$in": opt.ProjectNames}
	}
	query["is_archived"] = false
	query["is_deleted"] = false
	if opt.CreateTime > 0 {
		comparison := "$gte"
		if opt.BeforeCreatTime {
			comparison = "$lte"
		}
		query["create_time"] = bson.M{comparison: opt.CreateTime}
	}

	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"create_time", -1}})
	}

	return c.Collection.Find(context.TODO(), query, opts)
}
