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
	"time"

	timeutil "github.com/jinzhu/now"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types"
)

type ListTaskOption struct {
	PipelineName        string
	PipelineNames       []string
	Status              config.Status
	Statuses            []string
	Team                string
	TeamID              int
	Limit               int
	Skip                int
	Detail              bool
	ForWorkflowTaskList bool
	Type                config.PipelineType
	CreateTime          int64
	TaskCreator         string
	TaskCreators        []string
	Source              string
	Committers          []string
	ServiceModule       []string
	MergeRequestID      string
	NeedTriggerBy       bool
	NeedAllData         bool
}

type FindTaskOption struct {
	PipelineName string
	Status       config.Status
	Type         config.PipelineType
}

type CodeInfo struct {
	AuthorName    string `json:"author_name"`
	CommitId      string `json:"commit_id"`
	CommitMessage string `json:"commit_message"`
}

type ServiceModule struct {
	ServiceModule string              `json:"service_module"`
	ServiceName   string              `json:"service_name"`
	CodeInfo      []*types.Repository `json:"code_info"`
}

type BuildStage struct {
	SubTasks map[string]task.Build `bson:"sub_tasks"                    json:"-"`
}

type TaskPreview struct {
	TaskID         int64                     `bson:"task_id"               json:"task_id"`
	TaskCreator    string                    `bson:"task_creator"          json:"task_creator"`
	ProductName    string                    `bson:"product_name"          json:"product_name"`
	PipelineName   string                    `bson:"pipeline_name"         json:"pipeline_name"`
	Namespace      string                    `bson:"namespace"             json:"namespace"`
	ServiceName    string                    `bson:"service_name"          json:"service_name"`
	Status         config.Status             `bson:"status"                json:"status"`
	CreateTime     int64                     `bson:"create_time"           json:"create_time,omitempty"`
	StartTime      int64                     `bson:"start_time"            json:"start_time,omitempty"`
	EndTime        int64                     `bson:"end_time"              json:"end_time,omitempty"`
	SubTasks       []*map[string]interface{} `bson:"sub_tasks,omitempty"   json:"sub_tasks,omitempty"`
	TaskArgs       *models.TaskArgs          `bson:"task_args"             json:"task_args"`
	WorkflowArgs   *models.WorkflowTaskArgs  `bson:"workflow_args"         json:"workflow_args"`
	TestReports    map[string]interface{}    `bson:"test_reports,omitempty" json:"test_reports,omitempty"`
	Type           config.PipelineType       `bson:"type"                  json:"type"`
	Stages         []*models.Stage           `bson:"stages"                json:"stages,omitempty"`
	ServiceModules []*ServiceModule          `bson:"-"                     json:"service_modules"`
	TriggerBy      *models.TriggerBy         `bson:"trigger_by,omitempty"  json:"trigger_by,omitempty"`
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
		{
			Keys: bson.D{
				bson.E{Key: "pipeline_name", Value: 1},
				bson.E{Key: "is_archived", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetBackground(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

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

func (c *TaskColl) FindTask(pipelineName string, typeString config.PipelineType) (*task.Task, error) {
	query := bson.M{"pipeline_name": pipelineName, "type": typeString, "is_deleted": false}
	opts := options.FindOne().SetSort(bson.D{{"create_time", -1}})
	res := &task.Task{}
	err := c.FindOne(context.TODO(), query, opts).Decode(res)

	return res, err
}

func (c *TaskColl) FindLatestTask(args *FindTaskOption) (*task.Task, error) {
	query := bson.M{"is_deleted": false}

	if args.PipelineName != "" {
		query["pipeline_name"] = args.PipelineName
	}
	if args.Type != "" {
		query["type"] = args.Type
	}
	if args.Status != "" {
		query["status"] = args.Status
	}

	opts := options.FindOne().SetSort(bson.D{{"task_id", -1}})
	res := &task.Task{}
	err := c.FindOne(context.TODO(), query, opts).Decode(res)

	return res, err
}

type TaskInfo struct {
	ID           taskGrouped `bson:"_id"`
	TaskID       int64       `bson:"task_id"`
	PipelineName string      `bson:"pipeline_name"`
	Status       string      `bson:"status"`
}

type taskGrouped struct {
	PipelineName string              `bson:"pipeline_name"`
	Type         config.PipelineType `bson:"type"`
}

func (c *TaskColl) ListRecentTasks(args *ListTaskOption) ([]*TaskInfo, error) {
	query := bson.M{"is_deleted": false}
	if args.Type != "" {
		query["type"] = args.Type
	}
	if args.Status != "" {
		query["status"] = args.Status
	}
	if len(args.PipelineNames) > 0 {
		query["pipeline_name"] = bson.M{"$in": args.PipelineNames}
	}

	pipeline := []bson.M{
		{
			"$match": query,
		},
		{
			"$sort": bson.M{"task_id": -1},
		},
		{
			"$group": bson.M{
				"_id": bson.D{
					{"pipeline_name", "$pipeline_name"},
					{"type", "$type"},
				},
				"pipeline_name": bson.M{"$first": "$pipeline_name"},
				"task_id":       bson.M{"$first": "$task_id"},
				"status":        bson.M{"$first": "$status"},
			},
		},
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	res := make([]*TaskInfo, 0)
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
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
	if len(option.Statuses) > 0 {
		query["status"] = bson.M{"$in": option.Statuses}
	} else if option.Status != "" {
		query["status"] = option.Status
	}
	if option.Team != "" {
		query["team"] = option.Team
	}
	if len(option.TaskCreators) > 0 {
		query["task_creator"] = bson.M{"$in": option.TaskCreators}
	} else if option.TaskCreator != "" {
		query["task_creator"] = option.TaskCreator
	}
	if len(option.Committers) > 0 {
		query["workflow_args.committer"] = bson.M{"$in": option.Committers}
	}
	if len(option.ServiceModule) > 0 {
		query["workflow_args.targets.name"] = bson.M{"$in": option.ServiceModule}
	}
	if option.Source != "" {
		query["trigger_by.source"] = option.Source
	}
	if option.MergeRequestID != "" {
		query["trigger_by.merge_request_id"] = option.MergeRequestID
	}
	query["is_archived"] = false
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
		if option.Type == config.PipelineTypeService {
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

	// show necessary fields when listing workflow tasks
	if option.ForWorkflowTaskList {
		selector = append(selector, bson.E{"workflow_args.targets.name", 1})
		selector = append(selector, bson.E{"workflow_args.targets.service_name", 1})
		selector = append(selector, bson.E{"workflow_args.namespace", 1})
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
		return ret, err
	}

	if err = cursor.All(context.TODO(), &ret); err != nil {
		return nil, err
	}
	return
}

func (c *TaskColl) ListPreview(pipelineNames []string) (ret []*TaskPreview, err error) {
	ret = make([]*TaskPreview, 0)
	query := bson.M{}
	if pipelineNames != nil {
		query["pipeline_name"] = bson.M{"$in": pipelineNames}
	}
	query["is_archived"] = false
	query["is_deleted"] = false
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
	opt := options.Find()
	opt.SetProjection(selector)
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return ret, err
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
	Statuses      []string
	Status        config.TaskStatus
	TaskCreators  []string
	Committers    []string
	ServiceModule []string
	Type          config.PipelineType
}

func (c *TaskColl) Count(option *CountTaskOption) (ret int, err error) {
	if option == nil {
		return 0, errors.New("nil list pipeline option")
	}

	query := bson.M{}
	if len(option.TaskCreators) > 0 {
		query["task_creator"] = bson.M{"$in": option.TaskCreators}
	}
	if len(option.Committers) > 0 {
		query["workflow_args.committer"] = bson.M{"$in": option.Committers}
	}
	if len(option.ServiceModule) > 0 {
		query["workflow_args.targets.name"] = bson.M{"$in": option.ServiceModule}
	}
	if option.PipelineNames != nil {
		query["pipeline_name"] = bson.M{"$in": option.PipelineNames}
	}
	if len(option.Statuses) > 0 {
		query["status"] = bson.M{"$in": option.Statuses}
	} else if option.Status != "" {
		query["status"] = option.Status
	}
	if option.Type != "" {
		query["type"] = option.Type
	}
	query["is_archived"] = false
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

func (c *TaskColl) ListAllTasks(option *ListAllTaskOption) ([]*task.Task, error) {
	resp := make([]*task.Task, 0)
	if option == nil {
		return resp, errors.New("nil list pipeline option")
	}
	query := bson.M{"is_deleted": false}
	if len(option.ProductNames) > 0 {
		query["product_name"] = bson.M{"$in": option.ProductNames}
	}
	if option.Type != "" {
		query["type"] = option.Type
	}
	if option.CreateTime > 0 {
		comparison := "$gte"
		if option.BeforeCreatTime {
			comparison = "$lte"
		}
		query["create_time"] = bson.M{comparison: option.CreateTime}
	}
	opt := &options.FindOptions{}
	if option.Limit != 0 {
		projection := bson.D{
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
		opt.SetProjection(projection).SetSort(bson.D{{"create_time", -1}}).SetSkip(int64(option.Skip)).SetLimit(int64(option.Limit))
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
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

func (c *TaskColl) ArchiveHistoryPipelineTask(pipelineName string, taskType config.PipelineType, remain, remainDays int) error {
	if remain == 0 && remainDays == 0 {
		return nil
	}
	query := bson.M{"pipeline_name": pipelineName, "type": taskType, "is_deleted": false, "is_archived": false}
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

func (c *TaskColl) DistinctFieldsPipelineTask(fieldName, projectName, pipelineName string, typeString config.PipelineType, isNeedAllData bool) ([]interface{}, error) {
	query := bson.M{"product_name": projectName, "pipeline_name": pipelineName, "type": typeString, "is_deleted": false}
	if isNeedAllData {
		createTime := timeutil.BeginningOfDay().AddDate(0, -3, 0).Unix()
		query["create_time"] = bson.M{"$gt": createTime}
	}
	res, err := c.Distinct(context.TODO(), fieldName, query)
	return res, err
}

func (c *TaskColl) ListByCursor(option *ListAllTaskOption) (*mongo.Cursor, error) {
	if option == nil {
		return nil, errors.New("nil list pipeline option")
	}
	query := bson.M{"is_deleted": false}
	if option.ProductName != "" {
		query["product_name"] = option.ProductName
	}
	if len(option.ProductNames) > 0 {
		query["product_name"] = bson.M{"$in": option.ProductNames}
	}
	if option.Type != "" {
		query["type"] = option.Type
	}
	if option.CreateTime > 0 {
		comparison := "$gte"
		if option.BeforeCreatTime {
			comparison = "$lte"
		}
		query["create_time"] = bson.M{comparison: option.CreateTime}
	}
	return c.Collection.Find(context.TODO(), query)
}
