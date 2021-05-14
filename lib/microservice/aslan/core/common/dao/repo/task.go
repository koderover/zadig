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

	timeutil "github.com/jinzhu/now"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ListTaskOption struct {
	PipelineName   string
	Status         config.Status
	Team           string
	TeamID         int
	Limit          int
	Skip           int
	Detail         bool
	Type           config.PipelineType
	CreateTime     int64
	TaskCreator    string
	Source         string
	MergeRequestID string
	NeedTriggerBy  bool
	NeedAllData    bool
}

type TaskPreview struct {
	TaskID       int64                     `bson:"task_id"               json:"task_id"`
	TaskCreator  string                    `bson:"task_creator"          json:"task_creator"`
	ProductName  string                    `bson:"product_name"          json:"product_name"`
	PipelineName string                    `bson:"pipeline_name"         json:"pipeline_name"`
	Namespace    string                    `bson:"namespace"             json:"namespace"`
	ServiceName  string                    `bson:"service_name"          json:"service_name"`
	Status       config.Status             `bson:"status"                json:"status"`
	CreateTime   int64                     `bson:"create_time"           json:"create_time,omitempty"`
	StartTime    int64                     `bson:"start_time"            json:"start_time,omitempty"`
	EndTime      int64                     `bson:"end_time"              json:"end_time,omitempty"`
	SubTasks     []*map[string]interface{} `bson:"sub_tasks,omitempty"   json:"sub_tasks,omitempty"`
	TaskArgs     *models.TaskArgs          `bson:"task_args"             json:"task_args"`
	WorkflowArgs *models.WorkflowTaskArgs  `bson:"workflow_args"         json:"workflow_args"`
	TestReports  map[string]interface{}    `bson:"test_reports,omitempty" json:"test_reports,omitempty"`
	Type         config.PipelineType       `bson:"type"                  json:"type"`
	Stages       []*models.Stage           `bson:"stages"                json:"stages,omitempty"`
	// 服务名称，用于任务列表的展示
	BuildServices []string          `bson:"-"                     json:"build_services"`
	TriggerBy     *models.TriggerBy `bson:"trigger_by,omitempty"  json:"trigger_by,omitempty"`
}

type ListAllTaskOption struct {
	ProductNames    []string
	ProductName     string
	PipelineName    string
	Type            config.PipelineType
	CreateTime      int64
	BeforeCreatTime bool
	Limit           int
	Skip            int
}

type TaskColl struct {
	*mongo.Collection

	coll string
}

func NewTaskColl() *TaskColl {
	name := task.Task{}.TableName()
	return &TaskColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *TaskColl) GetCollectionName() string {
	return c.coll
}

func (c *TaskColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "task_id", Value: 1},
				bson.E{Key: "pipeline_name", Value: 1},
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
				bson.E{Key: "pipeline_name", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *TaskColl) DeleteByPipelineNameAndType(pipelineName string, typeString config.PipelineType) error {
	query := bson.M{"pipeline_name": pipelineName, "type": typeString}
	change := bson.M{"$set": bson.M{
		"is_deleted":  true,
		"is_archived": true,
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *TaskColl) UpdateStatus(taskID int64, pipelineName, userName string, status config.Status) error {
	query := bson.M{"task_id": taskID, "pipeline_name": pipelineName, "is_deleted": false}
	change := bson.M{"$set": bson.M{
		"status":       status,
		"task_revoker": userName,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *TaskColl) Find(id int64, pipelineName string, typeString config.PipelineType) (*task.Task, error) {
	query := bson.M{"task_id": id, "pipeline_name": pipelineName, "type": typeString, "is_deleted": false}

	res := &task.Task{}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *TaskColl) FindTask(pipelineName, namespace string, typeString config.PipelineType) (*task.Task, error) {
	query := bson.M{"namespace": namespace, "pipeline_name": pipelineName, "type": typeString, "is_deleted": false}
	opts := options.FindOne().SetSort(bson.D{{"create_time", -1}})
	res := &task.Task{}
	err := c.FindOne(context.TODO(), query, opts).Decode(res)

	return res, err
}

func (c *TaskColl) List(option *ListTaskOption) (ret []*TaskPreview, err error) {
	ret = make([]*TaskPreview, 0)
	if option == nil {
		return ret, errors.New("nil list pipeline option")
	}

	query := bson.M{}
	if !option.NeedAllData {
		// 仅支持查询最近三个月的数据
		createTime := timeutil.BeginningOfDay().AddDate(0, -3, 0).Unix()
		query["create_time"] = bson.M{"$gt": createTime}
	}

	if option.PipelineName != "" {
		query["pipeline_name"] = option.PipelineName
	}
	if option.Status != "" {
		query["status"] = option.Status
	}
	if option.Team != "" {
		query["team"] = option.Team
	}
	if option.TaskCreator != "" {
		query["task_creator"] = option.TaskCreator
	}
	if option.Source != "" {
		query["trigger_by.source"] = option.Source
	}
	if option.MergeRequestID != "" {
		query["trigger_by.merge_request_id"] = option.MergeRequestID
	}
	query["is_deleted"] = false

	//是否需要subtask信息
	selector := bson.D{
		{"task_id", 1},
		{"task_creator", 1},
		{"product_name", 1},
		{"pipeline_name", 1},
		{"status", 1},
		{"create_time", 1},
		{"start_time", 1},
		{"end_time", 1},
		{"type", 1},
	}

	if option.NeedTriggerBy {
		selector = append(selector, bson.E{"trigger_by", 1})
	}

	if option.Type != "" {
		query["type"] = option.Type
		if option.Type == config.ServiceType {
			selector = append(selector, bson.E{"namespace", 1})
			selector = append(selector, bson.E{"service_name", 1})
		}
	}

	if option.Detail {
		selector = append(selector, bson.E{"sub_tasks", 1})
		selector = append(selector, bson.E{"task_args", 1})
		selector = append(selector, bson.E{"workflow_args", 1})
		selector = append(selector, bson.E{"test_reports", 1})
		selector = append(selector, bson.E{"stages", 1})
	}
	opt := options.Find()
	opt.SetSort(bson.D{{"create_time", -1}})
	if option.Limit != 0 {
		opt.SetProjection(selector).SetSkip(int64(option.Skip)).SetLimit(int64(option.Limit))
	} else if option.CreateTime > 0 {
		selector = append(selector, bson.E{"stages", 1})

		query["type"] = config.WorkflowType
		query["stages"] = bson.M{"$elemMatch": bson.M{"type": "buildv2"}}
		query["create_time"] = bson.M{"$gt": option.CreateTime}
		opt.SetProjection(selector)
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return
	}
	err = cursor.All(context.TODO(), &ret)
	return
}

func (c *TaskColl) Update(args *task.Task) error {
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	query := bson.M{"task_id": args.TaskID, "pipeline_name": args.PipelineName, "is_deleted": false}
	change := bson.M{"$set": bson.M{
		"task_creator":     args.TaskCreator,
		"status":           args.Status,
		"task_revoker":     args.TaskRevoker,
		"start_time":       args.StartTime,
		"end_time":         args.EndTime,
		"sub_tasks":        args.SubTasks,
		"req_id":           args.ReqID,
		"agent_host":       args.AgentHost,
		"task_args":        args.TaskArgs,
		"target":           args.Target,
		"build_module_ver": args.BuildModuleVer,
		"stages":           args.Stages,
		"workflow_args":    args.WorkflowArgs,
		"test_reports":     args.TestReports,
		"is_restart":       args.IsRestart,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *TaskColl) Rename(oldName, newName string, taskType config.PipelineType) error {
	query := bson.M{"pipeline_name": oldName, "type": taskType, "is_deleted": false}

	change := bson.M{"$set": bson.M{
		"pipeline_name": newName,
	}}

	_, err := c.UpdateMany(context.Background(), query, change)
	return err
}

func (c *TaskColl) Create(args *task.Task) error {
	if args == nil {
		return errors.New("nil PipelineTaskV2")
	}

	now := time.Now().Unix()
	args.CreateTime = now
	args.StartTime = now
	args.EndTime = now

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type CountTaskOption struct {
	PipelineNames []string
	Status        config.TaskStatus
	Type          config.PipelineType
}

func (c *TaskColl) Count(option *CountTaskOption) (ret int, err error) {
	if option == nil {
		return 0, errors.New("nil list pipeline option")
	}

	query := bson.M{}
	if option.PipelineNames != nil {
		query["pipeline_name"] = bson.M{"$in": option.PipelineNames}
	}
	if option.Status != "" {
		query["status"] = option.Status
	}
	if option.Type != "" {
		query["type"] = option.Type
	}
	query["is_deleted"] = false

	// 仅支持查询最近三个月的数据
	createTime := timeutil.BeginningOfDay().AddDate(0, -3, 0).Unix()
	query["create_time"] = bson.M{"$gt": createTime}

	count, err := c.CountDocuments(context.TODO(), query)
	ret = int(count)
	return
}

func (c *TaskColl) InCompletedTasks() ([]*task.Task, error) {
	ret := make([]*task.Task, 0)
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

func (c *TaskColl) FindTodoTasks() ([]*task.Task, error) {
	ret := make([]*task.Task, 0)
	query := bson.M{"status": bson.M{"$in": []string{"waiting", "quened", "created", "running", "blocked"}}}
	query["is_deleted"] = false

	opt := options.Find()
	opt.SetSort(bson.D{{"create_time", 1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &ret)

	return ret, err
}

func (c *TaskColl) ListTasks(option *ListAllTaskOption) ([]*task.Task, error) {
	ret := make([]*task.Task, 0)
	if option == nil {
		return ret, errors.New("nil list pipeline option")
	}
	query := bson.M{"is_deleted": false}
	if option.Type != "" {
		query["type"] = option.Type
	}
	if option.ProductName != "" {
		query["product_name"] = option.ProductName
	}
	if option.PipelineName != "" {
		query["pipeline_name"] = option.PipelineName
	}
	if option.CreateTime > 0 {
		comparison := "$gte"
		if option.BeforeCreatTime {
			comparison = "$lte"
		}
		query["create_time"] = bson.M{comparison: option.CreateTime}
	}

	ctx := context.Background()
	opts := options.Find()
	opts.SetSort(bson.D{{"create_time", -1}})
	opts.SetSkip(int64(option.Skip))
	opts.SetLimit(int64(option.Limit))
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, err
}
