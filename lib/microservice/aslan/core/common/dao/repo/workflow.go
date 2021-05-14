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
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ListWorkflowOption struct {
	ProductName string
	IsSort      bool
	TestName    string
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

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *WorkflowColl) List(opt *ListWorkflowOption) ([]*models.Workflow, error) {
	query := bson.M{}
	if opt.ProductName != "" {
		query["product_tmpl_name"] = opt.ProductName
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
