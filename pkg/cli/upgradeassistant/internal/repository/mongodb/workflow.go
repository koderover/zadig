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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ListWorkflowOption struct {
	IsSort   bool
	Projects []string
	Names    []string
	Ids      []string
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

func (c *WorkflowColl) List(opt *ListWorkflowOption) ([]*models.Workflow, error) {
	query := bson.M{}
	if len(opt.Projects) > 0 {
		query["product_tmpl_name"] = bson.M{"$in": opt.Projects}
	}
	if len(opt.Names) > 0 {
		query["name"] = bson.M{"$in": opt.Names}
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
