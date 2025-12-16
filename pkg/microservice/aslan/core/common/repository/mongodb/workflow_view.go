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
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type WorkflowViewColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowViewColl() *WorkflowViewColl {
	name := models.WorkflowView{}.TableName()
	return &WorkflowViewColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkflowViewColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowViewColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "project_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *WorkflowViewColl) Create(args *models.WorkflowView) error {
	if args == nil {
		return errors.New("nil workflow view args")
	}

	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *WorkflowViewColl) Update(args *models.WorkflowView) error {
	if args == nil {
		return errors.New("nil workflow view args")
	}
	if args.ID.IsZero() {
		return errors.New("empty workflow view id")
	}
	filter := bson.M{"_id": args.ID}
	update := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *WorkflowViewColl) ListNamesByProject(projectName string) ([]string, error) {
	resp := make([]string, 0)
	query := bson.M{"project_name": projectName}

	viewNames, err := c.Collection.Distinct(context.TODO(), "name", query)
	if err != nil {
		return resp, err
	}
	for _, name := range viewNames {
		switch name := name.(type) {
		case string:
			resp = append(resp, name)
		}
	}
	sort.Strings(resp)
	return resp, nil
}

func (c *WorkflowViewColl) ListByProject(projectName string) ([]*models.WorkflowView, error) {
	resp := make([]*models.WorkflowView, 0)
	ctx := context.Background()
	query := bson.M{"project_name": projectName}
	opts := options.Find().SetSort(bson.D{{"name", -1}})

	cur, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return resp, err
	}
	if err := cur.All(ctx, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *WorkflowViewColl) FindByID(idString string) (*models.WorkflowView, error) {
	resp := new(models.WorkflowView)
	id, err := primitive.ObjectIDFromHex(idString)
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

func (c *WorkflowViewColl) Find(projectName, viewName string) (*models.WorkflowView, error) {
	resp := new(models.WorkflowView)

	query := bson.M{}

	if projectName != "" {
		query["project_name"] = projectName
	}

	if viewName != "" {
		query["name"] = viewName
	}

	if err := c.FindOne(context.TODO(), query).Decode(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowViewColl) DeleteByProject(projectName, viewName string) error {
	query := bson.M{}

	if projectName != "" {
		query["project_name"] = projectName
	}

	if viewName != "" {
		query["name"] = viewName
	}

	_, err := c.Collection.DeleteMany(context.TODO(), query)
	if err != nil {
		return err
	}
	return nil
}

func (c *WorkflowViewColl) DeleteByID(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	if _, err := c.DeleteOne(context.TODO(), query); err != nil {
		return err
	}
	return nil
}
