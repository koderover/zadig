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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ListWorkflowOption struct {
	IsSort      bool
	Projects    []string
	Names       []string
	Ids         []string
	DisplayName string
}

type Workflow struct {
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

type ListWorkflowOpt struct {
	Workflows []Workflow
}

func (c *WorkflowColl) ListByWorkflows(opt ListWorkflowOpt) ([]*models.Workflow, error) {
	var res []*models.Workflow

	if len(opt.Workflows) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, workflow := range opt.Workflows {
		condition = append(condition, bson.M{
			"name":              workflow.Name,
			"product_tmpl_name": workflow.ProjectName,
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

type WorkflowColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowColl() *WorkflowColl {
	name := models.Workflow{}.TableName()
	return &WorkflowColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkflowColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *WorkflowColl) List(opt *ListWorkflowOption) ([]*models.Workflow, error) {
	query := bson.M{}
	if len(opt.Projects) > 0 {
		query["product_tmpl_name"] = bson.M{"$in": opt.Projects}
	}
	if len(opt.Names) > 0 {
		query["name"] = bson.M{"$in": opt.Names}
	}
	if opt.DisplayName != "" {
		query["display_name"] = opt.DisplayName
	}
	if len(opt.Ids) > 0 {
		var oids []primitive.ObjectID
		for _, id := range opt.Ids {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}

	resp := make([]*models.Workflow, 0)
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"update_time", -1}})
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

func (c *WorkflowColl) Count() (int64, error) {
	query := bson.M{}

	ctx := context.Background()
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *WorkflowColl) ListByTestName(testName string) ([]*models.Workflow, error) {
	// 查询test_stage.tests中存在此testName 或者 test_stage.test_names存在此testName的工作流
	match := []bson.M{
		{
			"test_stage.tests.test_name": testName,
		},
		{
			"test_stage.test_names": bson.M{
				"$elemMatch": bson.M{"$eq": testName},
			},
		},
	}

	query := bson.M{"$or": match}

	var resp []*models.Workflow
	ctx := context.Background()
	opts := options.Find()
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

func (c *WorkflowColl) Find(name string) (*models.Workflow, error) {
	res := &models.Workflow{}
	query := bson.M{"name": name}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *WorkflowColl) Delete(name string) error {
	query := bson.M{"name": name}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *WorkflowColl) Create(args *models.Workflow) error {
	if args == nil {
		return errors.New("nil Workflow args")
	}
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *WorkflowColl) BulkCreate(args []*models.Workflow) error {
	if len(args) == 0 {
		return nil
	}
	var ois []interface{}
	for _, arg := range args {
		arg.CreateTime = time.Now().Unix()
		arg.UpdateTime = time.Now().Unix()
		ois = append(ois, arg)
	}

	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

func (c *WorkflowColl) Replace(args *models.Workflow) error {
	if args == nil {
		return errors.New("nil Workflow args")
	}

	query := bson.M{"name": args.Name}

	args.UpdateTime = time.Now().Unix()

	_, err := c.ReplaceOne(context.TODO(), query, args)
	return err
}

func (c *WorkflowColl) ListWithScheduleEnabled() ([]*models.Workflow, error) {
	resp := make([]*models.Workflow, 0)
	query := bson.M{"schedule_enabled": true}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *WorkflowColl) ListWorkflowsByProjects(projects []string) ([]*models.Workflow, error) {
	resp := make([]*models.Workflow, 0)
	query := bson.M{}
	if len(projects) != 0 {
		if len(projects) != 1 || projects[0] != "*" {
			query = bson.M{"product_tmpl_name": bson.M{
				"$in": projects,
			}}
		}
	} else {
		return resp, nil
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *WorkflowColl) ListByCursor(opt *ListWorkflowOption) (*mongo.Cursor, error) {
	query := bson.M{}
	if len(opt.Projects) > 0 {
		query["product_tmpl_name"] = bson.M{"$in": opt.Projects}
	}
	if len(opt.Names) > 0 {
		query["name"] = bson.M{"$in": opt.Names}
	}
	if opt.DisplayName != "" {
		query["display_name"] = opt.DisplayName
	}
	if len(opt.Ids) > 0 {
		var oids []primitive.ObjectID
		for _, id := range opt.Ids {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	return c.Collection.Find(ctx, query, opts)
}
