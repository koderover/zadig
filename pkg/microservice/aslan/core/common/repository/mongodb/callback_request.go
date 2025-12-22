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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CallbackRequestColl struct {
	*mongo.Collection

	coll string
}

func NewCallbackRequestColl() *CallbackRequestColl {
	name := models.CallbackRequest{}.TableName()
	return &CallbackRequestColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *CallbackRequestColl) GetCollectionName() string {
	return c.coll
}

func (c *CallbackRequestColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "task_id", Value: 1},
			bson.E{Key: "task_name", Value: 1},
			bson.E{Key: "project_name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *CallbackRequestColl) Create(req *models.CallbackRequest) error {
	if req == nil {
		return errors.New("nil callbackRequest args")
	}

	_, err := c.Collection.InsertOne(context.TODO(), req)

	return err
}

type CallbackFindOption struct {
	TaskID       int64
	PipelineName string
	ProjectName  string
}

func (c *CallbackRequestColl) Find(req *CallbackFindOption) (*models.CallbackRequest, error) {
	if req == nil {
		return nil, errors.New("nil FindOption")
	}

	query := bson.M{}
	if req.TaskID != 0 {
		query["task_id"] = req.TaskID
	}
	if req.PipelineName != "" {
		query["task_name"] = req.PipelineName
	}
	if req.ProjectName != "" {
		query["project_name"] = req.ProjectName
	}

	resp := new(models.CallbackRequest)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
