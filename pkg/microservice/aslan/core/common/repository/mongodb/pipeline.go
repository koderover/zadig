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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type PipelineListOption struct {
	IsPreview   bool
	Targets     []string
	ProductName string
}

type PipelineFindOption struct {
	Name string
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

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *PipelineColl) List(opt *PipelineListOption) ([]*models.Pipeline, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	var resp []*models.Pipeline
	query := bson.M{"is_deleted": false}

	if opt.ProductName != "" {
		query["product_name"] = opt.ProductName
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
			{"target", 1},
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
	query := bson.M{"name": opt.Name, "is_deleted": false}
	err := c.FindOne(context.TODO(), query).Decode(resp)

	return resp, err
}

func (c *PipelineColl) Delete(name string) error {
	query := bson.M{"name": name}
	_, err := c.DeleteMany(context.TODO(), query)

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
