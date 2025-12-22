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

package vm

import (
	"context"
	"errors"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type VMJobColl struct {
	*mongo.Collection

	coll string
}

func NewVMJobColl() *VMJobColl {
	name := vm.VMJob{}.TableName()
	return &VMJobColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *VMJobColl) GetCollectionName() string {
	return c.coll
}

func (c *VMJobColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "status", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, indexes, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *VMJobColl) Create(obj *vm.VMJob) error {
	if obj == nil {
		return nil
	}

	result, err := c.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return errors.New("Failed to convert inserted ID to ObjectID")
	}

	obj.ID = insertedID
	return nil
}

func (c *VMJobColl) Update(idString string, obj *vm.VMJob) error {
	if obj == nil {
		return nil
	}

	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	change := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.Background(), query, change)
	return err
}

func (e *VMJobColl) UpdateStatus(idString string, status string) error {
	if status == "" {
		return nil
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{"status": status}}

	_, err = e.UpdateOne(context.Background(), query, change)
	return err
}

func (c *VMJobColl) FindByID(idString string) (*vm.VMJob, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": id}

	res := &vm.VMJob{}
	err = c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

type VMJobFindOption struct {
	JobID        int
	TaskID       int64
	ProjectName  string
	WorkflowName string
	JobName      string
}

func (c *VMJobColl) FindByOpts(opts VMJobFindOption) (*vm.VMJob, error) {
	query := bson.M{}
	if opts.JobID > 0 {
		query["_id"] = opts.JobID
	}
	if opts.TaskID > 0 {
		query["task_id"] = opts.TaskID
	}
	if opts.ProjectName != "" {
		query["project_name"] = opts.ProjectName
	}
	if opts.WorkflowName != "" {
		query["workflow_name"] = opts.WorkflowName
	}
	if opts.JobName != "" {
		query["job_name"] = opts.JobName
	}
	query["is_deleted"] = false

	res := &vm.VMJob{}
	err := c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

func (c *VMJobColl) FindOldestByTags(labels []string) (*vm.VMJob, error) {
	query := bson.M{
		"vm_labels": bson.M{"$in": labels},
		"status":    string(config.StatusCreated),
	}

	opts := options.FindOne()
	opts.SetSort(bson.M{"created_time": 1})

	res := &vm.VMJob{}
	err := c.FindOne(context.Background(), query, opts).Decode(res)
	return res, err
}

type VMJobOpts struct {
	Names  []string
	Status string
}

func (c *VMJobColl) ListByOpts(opts *VMJobOpts) ([]*vm.VMJob, error) {
	query := bson.M{}
	if len(opts.Names) > 0 {
		query["name"] = bson.M{"$in": opts.Names}
	}
	if opts.Status != "" {
		query["status"] = opts.Status
	}

	res := make([]*vm.VMJob, 0)
	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.Background(), &res)

	// sort by created time
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedTime < res[j].CreatedTime
	})
	return res, err
}

func (c *VMJobColl) DeleteByID(idString string, status string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{
		"_id": id,
	}

	change := bson.M{
		"$set": bson.M{
			"status":     status,
			"is_deleted": true,
		},
	}

	_, err = c.UpdateOne(context.Background(), query, change)
	return err
}
