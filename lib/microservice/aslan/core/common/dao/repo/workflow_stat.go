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

type WorkflowStatColl struct {
	*mongo.Collection

	coll string
}

func NewWorkflowStatColl() *WorkflowStatColl {
	name := models.WorkflowStat{}.TableName()
	return &WorkflowStatColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowStatColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "type", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *WorkflowStatColl) Create(args *models.WorkflowStat) error {
	if args == nil {
		return errors.New("nil workflow stat args")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *WorkflowStatColl) Upsert(args *models.WorkflowStat) error {
	if args == nil {
		return errors.New("nil workflow stat args")
	}

	query := bson.M{"name": args.Name, "type": args.Type}
	args.UpdatedAt = time.Now().Unix()
	change := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))

	return err
}

func (c *WorkflowStatColl) Delete(name, workflowType string) error {
	query := bson.M{"name": name, "type": workflowType}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

type WorkflowStatArgs struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (c *WorkflowStatColl) FindWorkflowStat(args *WorkflowStatArgs) ([]*models.WorkflowStat, error) {
	workflowStats := make([]*models.WorkflowStat, 0)
	if args == nil {
		return workflowStats, errors.New("nil workflow stat args")
	}

	query := bson.M{}
	if args.Name != "" {
		query["name"] = args.Name
	}

	if args.Type != "" {
		query["type"] = args.Type
	}

	ctx := context.Background()

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &workflowStats)
	if err != nil {
		return nil, err
	}

	return workflowStats, err
}
