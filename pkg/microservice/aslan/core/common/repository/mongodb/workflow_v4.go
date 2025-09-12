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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm/utils"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type WorkflowV4Coll struct {
	*mongo.Collection

	coll string
}

type ListWorkflowV4Option struct {
	ProjectName    string
	DisplayName    string
	Names          []string
	Category       setting.WorkflowCategory
	JobTypes       []config.JobType
	Infrastructure string
}

func NewWorkflowV4Coll() *WorkflowV4Coll {
	name := models.WorkflowV4{}.TableName()
	return &WorkflowV4Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV4Coll) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV4Coll) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project", Value: 1},
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project", Value: 1},
				bson.E{Key: "display_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

type WorkflowV4 struct {
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

type ListWorkflowV4Opt struct {
	Workflows []WorkflowV4
}

func (c *WorkflowV4Coll) ListByWorkflows(opt ListWorkflowV4Opt) ([]*models.WorkflowV4, error) {
	var res []*models.WorkflowV4

	if len(opt.Workflows) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, workflow := range opt.Workflows {
		condition = append(condition, bson.M{
			"name":    workflow.Name,
			"project": workflow.ProjectName,
		})
	}
	filter := bson.D{{"$or", condition}}
	cursor, err := c.Collection.Find(context.TODO(), filter)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *WorkflowV4Coll) ListByProjectNames(projects []string) ([]*models.WorkflowV4, error) {
	resp := make([]*models.WorkflowV4, 0)
	query := bson.M{}
	if len(projects) != 0 {
		if len(projects) != 1 || projects[0] != "*" {
			query = bson.M{"project": bson.M{
				"$in": projects,
			}}
		}
	} else {
		return resp, nil
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *WorkflowV4Coll) BulkCreate(args []*models.WorkflowV4) error {
	if len(args) == 0 {
		return nil
	}
	var ois []interface{}
	for _, arg := range args {
		arg.CreateTime = time.Now().Unix()
		arg.UpdateTime = time.Now().Unix()
		arg.UpdateHash()
		ois = append(ois, arg)
	}

	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

func (c *WorkflowV4Coll) Create(obj *models.WorkflowV4) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object")
	}

	obj.UpdateHash()

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

type SetCustomFieldsOptions struct {
	ProjectName  string
	WorkflowName string
}

func (c *WorkflowV4Coll) SetCustomFields(opts SetCustomFieldsOptions, args *models.CustomField) error {
	if args == nil {
		return nil
	}

	query := bson.M{"project": opts.ProjectName, "name": opts.WorkflowName}
	change := bson.M{"$set": bson.M{"custom_field": args}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

type WorkFlowOptions struct {
	ProjectName  string
	WorkflowName string
}

func (c *WorkflowV4Coll) FindByOptions(opts WorkFlowOptions) (*models.WorkflowV4, error) {
	workflow := &models.WorkflowV4{}
	query := bson.M{"project": opts.ProjectName, "name": opts.WorkflowName}
	err := c.FindOne(context.Background(), query).Decode(&workflow)
	if err != nil {
		return nil, err
	}
	return workflow, nil
}

func (c *WorkflowV4Coll) Count() (int64, error) {
	query := bson.M{}

	ctx := context.Background()
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *WorkflowV4Coll) List(opt *ListWorkflowV4Option, pageNum, pageSize int64) ([]*models.WorkflowV4, int64, error) {
	resp := make([]*models.WorkflowV4, 0)
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project"] = opt.ProjectName
	}
	if opt.DisplayName != "" {
		query["display_name"] = opt.DisplayName
	}
	if len(opt.Names) > 0 {
		query["name"] = bson.M{"$in": opt.Names}
	}
	if opt.Category != "" {
		query["category"] = opt.Category
	}
	if len(opt.JobTypes) > 0 {
		query["stages.jobs.type"] = bson.M{"$in": opt.JobTypes}
	}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, count, err
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
		return nil, count, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, count, err
	}
	return resp, count, nil
}

type ListWorkflowV4InGlobalOption struct {
	ProjectName           string
	ProjectNames          []string
	FavoriteWorkflowNames []string
	CollModeWorkflowNames []string
	PageNum               int64
	PageSize              int64
	SortBy                setting.ListWorkflowV4InGlobalSortBy
	OrderBy               setting.ListWorkflowV4InGlobalOrderBy
}

func (c *WorkflowV4Coll) ListInGlobal(opt *ListWorkflowV4InGlobalOption) ([]*models.WorkflowV4, int64, error) {
	resp := make([]*models.WorkflowV4, 0)
	query := bson.M{}

	// 如果存在 FavoriteWorkflowNames，只查询这些工作流，忽略其他条件
	if len(opt.FavoriteWorkflowNames) > 0 {
		query["name"] = bson.M{"$in": opt.FavoriteWorkflowNames}
	} else {
		// 构建查询条件
		var conditions bson.A
		if opt.ProjectName != "" {
			conditions = append(conditions, bson.M{"project": opt.ProjectName})
		} else if len(opt.ProjectNames) > 0 {
			conditions = append(conditions, bson.M{"project": bson.M{"$in": opt.ProjectNames}})
		}
		if len(opt.CollModeWorkflowNames) > 0 {
			conditions = append(conditions, bson.M{"name": bson.M{"$in": opt.CollModeWorkflowNames}})
		}

		// 根据条件数量构建查询
		if len(conditions) == 1 {
			// 只有一个条件，直接合并到主查询中
			for k, v := range conditions[0].(bson.M) {
				query[k] = v
			}
		} else if len(conditions) > 1 {
			// 多个条件，使用 OR 连接
			query["$or"] = conditions
		}
	}

	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, count, err
	}

	var findOption *options.FindOptions
	if opt.PageSize == 0 {
		opt.PageSize = 10
	}
	if opt.PageNum == 0 {
		opt.PageNum = 1
	}

	findOption = options.Find().
		SetSkip((opt.PageNum - 1) * opt.PageSize).
		SetLimit(opt.PageSize)

	if opt.SortBy != "" && opt.OrderBy != 0 {
		findOption.SetSort(bson.D{
			bson.E{Key: string(opt.SortBy), Value: opt.OrderBy},
		})
	}

	log.Debugf("query: %+v", query)
	log.Debugf("skip: %d, limit: %d", *findOption.Skip, *findOption.Limit)
	log.Debugf("sort: %v, orderBy: %d", findOption.Sort, opt.OrderBy)

	cursor, err := c.Collection.Find(context.TODO(), query, findOption)
	if err != nil {
		return nil, count, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, count, err
	}
	return resp, count, nil
}

func (c *WorkflowV4Coll) Find(name string) (*models.WorkflowV4, error) {
	resp := new(models.WorkflowV4)
	query := bson.M{"name": name}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowV4Coll) GetByID(idstring string) (*models.WorkflowV4, error) {
	resp := new(models.WorkflowV4)
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

func (c *WorkflowV4Coll) Update(idString string, obj *models.WorkflowV4) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	obj.UpdateHash()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *WorkflowV4Coll) DeleteByID(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *WorkflowV4Coll) ListByCursor(opt *ListWorkflowV4Option) (*mongo.Cursor, error) {
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project"] = opt.ProjectName
	}
	if opt.DisplayName != "" {
		query["display_name"] = opt.DisplayName
	}
	if len(opt.Names) > 0 {
		query["name"] = bson.M{"$in": opt.Names}
	}
	if len(opt.JobTypes) > 0 {
		query["stages.jobs.type"] = bson.M{"$in": opt.JobTypes}
	}

	return c.Collection.Find(context.TODO(), query)
}

func (c *WorkflowV4Coll) GetJobNameList(projectName, workflowName, jobType string) ([]string, error) {
	workflow := new(models.WorkflowV4)
	query := bson.M{"project": projectName, "name": workflowName}

	err := c.FindOne(context.Background(), query).Decode(&workflow)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if string(job.JobType) == jobType {
				if !utils.Contains(names, job.Name) {
					names = append(names, job.Name)
				}
			}
		}
	}
	return names, nil
}
