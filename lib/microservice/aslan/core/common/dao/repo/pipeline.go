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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type PipelineListOption struct {
	Teams          []string
	TeamName       []string
	IsDeleted      bool
	IsPreview      bool
	Targets        []string
	BuildModuleVer string
	ProductName    string
}

type PipelineFindOption struct {
	Name      string
	IsDeleted bool
}

type PipelineColl struct {
	*mongo.Collection

	coll string
}

func NewPipelineColl() *PipelineColl {
	name := models.Pipeline{}.TableName()
	return &PipelineColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *PipelineColl) GetCollectionName() string {
	return c.coll
}

func (c *PipelineColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys:    bson.M{"name": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "build_module_ver", Value: 1},
				bson.E{Key: "target", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *PipelineColl) List(opt *PipelineListOption) ([]*models.Pipeline, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	var resp []*models.Pipeline
	query := bson.M{"is_deleted": opt.IsDeleted}

	if len(opt.Teams) != 0 {
		query["team"] = bson.M{"$in": opt.Teams}
	}
	if opt.ProductName != "" {
		query["product_name"] = opt.ProductName
	}
	if len(opt.BuildModuleVer) != 0 {
		query["build_module_ver"] = opt.BuildModuleVer
	}
	if len(opt.Targets) != 0 {
		query["target"] = bson.M{"$in": opt.Targets}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsPreview {
		projection := bson.D{
			{"name", 1},
			{"pipeline_name", 1},
			{"type", 1},
			{"team", 1},
			{"target", 1},
			{"build_module_ver", 1},
			{"enabled", 1},
		}
		opts.SetProjection(projection)
	}
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

func (c *PipelineColl) Find(opt *PipelineFindOption) (*models.Pipeline, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}

	resp := &models.Pipeline{}
	query := bson.M{"name": opt.Name, "is_deleted": opt.IsDeleted}
	err := c.FindOne(context.TODO(), query).Decode(resp)

	return resp, err
}

func (c *PipelineColl) Delete(name string) error {
	query := bson.M{"name": name}
	change := bson.M{"$set": bson.M{
		"is_deleted":  true,
		"update_time": time.Now().Unix(),
	}}
	_, err := c.UpdateMany(context.TODO(), query, change)

	return err
}

func (c *PipelineColl) Upsert(args *models.Pipeline) error {
	if args == nil {
		return errors.New("nil Pipeline args")
	}

	query := bson.M{"name": args.Name, "is_deleted": false}

	change := bson.M{"$set": bson.M{
		"product_name":     args.ProductName,
		"description":      args.Description,
		"notifiers":        args.Notifiers,
		"update_by":        args.UpdateBy,
		"create_time":      time.Now().Unix(),
		"update_time":      time.Now().Unix(),
		"enabled":          args.Enabled,
		"schedules":        args.Schedules,
		"sub_tasks":        args.SubTasks,
		"hook":             args.Hook,
		"slack":            args.Slack,
		"multi_run":        args.MultiRun,
		"target":           args.Target,
		"build_module_ver": args.BuildModuleVer,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *PipelineColl) Rename(oldName, newName string) error {
	query := bson.M{"name": oldName, "is_deleted": false}

	change := bson.M{"$set": bson.M{
		"name": newName,
	}}

	_, err := c.UpdateOne(context.Background(), query, change)
	return err
}

func (c *TaskColl) UpdateUnfinishedTask(args *task.Task) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	condition := bson.M{"$nin": []string{
		string(config.StatusPassed),
		string(config.StatusFailed),
		string(config.StatusTimeout),
	}}
	query := bson.M{"task_id": args.TaskID, "pipeline_name": args.PipelineName, "is_deleted": false, "status": condition}
	change := bson.M{"$set": bson.M{
		"task_creator":  args.TaskCreator,
		"status":        args.Status,
		"task_revoker":  args.TaskRevoker,
		"start_time":    args.StartTime,
		"end_time":      args.EndTime,
		"sub_tasks":     args.SubTasks,
		"req_id":        args.ReqID,
		"agent_host":    args.AgentHost,
		"task_args":     args.TaskArgs,
		"workflow_args": args.WorkflowArgs,
		"stages":        args.Stages,
		"test_reports":  args.TestReports,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *TaskColl) ArchiveHistoryPipelineTask(pipelineName string, taskType config.PipelineType, remain int) error {
	query := bson.M{"pipeline_name": pipelineName, "type": taskType, "is_deleted": false}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return err
	}
	query["task_id"] = bson.M{"$lt": int(count) - remain + 1}
	change := bson.M{"$set": bson.M{
		"is_archived": true,
	}}
	_, err = c.UpdateMany(context.TODO(), query, change)

	return err
}
