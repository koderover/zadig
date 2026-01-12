/*
Copyright 2022 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type SprintWorkItemTaskQueryOption struct {
	ID   string
}

type SprintWorkItemTaskListOption struct {
	PageNum      int64
	PageSize     int64
	WorkflowName string
	ID           string
	Status       []config.Status
}

type SprintWorkItemTaskColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewSprintWorkItemTaskColl() *SprintWorkItemTaskColl {
	name := models.SprintWorkItemTask{}.TableName()
	return &SprintWorkItemTaskColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewSprintWorkItemTaskCollWithSession(session mongo.Session) *SprintWorkItemTaskColl {
	name := models.SprintWorkItemTask{}.TableName()
	return &SprintWorkItemTaskColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *SprintWorkItemTaskColl) GetCollectionName() string {
	return c.coll
}

func (c *SprintWorkItemTaskColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "sprint_workitem_ids", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "status", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}
	_, err := c.Indexes().CreateMany(mongotool.SessionContext(ctx, c.Session), mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *SprintWorkItemTaskColl) Create(ctx *handler.Context, obj *models.SprintWorkItemTask) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	obj.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(mongotool.SessionContext(ctx, c.Session), obj)
	return err
}

func (c *SprintWorkItemTaskColl) Update(ctx *handler.Context, obj *models.SprintWorkItemTask) error {
	query := bson.M{"_id": obj.ID}
	change := bson.M{"$set": obj}
	_, err := c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemTaskColl) GetByID(ctx *handler.Context, idStr string) (*models.SprintWorkItemTask, error) {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, err
	}
	query := bson.M{
		"_id": id,
	}

	task := new(models.SprintWorkItemTask)
	if err := c.Collection.FindOne(mongotool.SessionContext(ctx, c.Session), query).Decode(task); err != nil {
		return nil, err
	}

	return task, nil
}

func (c *SprintWorkItemTaskColl) Find(ctx *handler.Context, opt *SprintWorkItemTaskQueryOption) (*models.SprintWorkItemTask, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}
	query := bson.M{}
	if len(opt.ID) > 0 {
		id, err := primitive.ObjectIDFromHex(opt.ID)
		if err != nil {
			return nil, err
		}
		query["_id"] = id
	}
	resp := new(models.SprintWorkItemTask)
	err := c.Collection.FindOne(mongotool.SessionContext(ctx, c.Session), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *SprintWorkItemTaskColl) List(ctx *handler.Context, option *SprintWorkItemTaskListOption) ([]*models.SprintWorkItemTask, int64, error) {
	query := bson.M{}
	resp := make([]*models.SprintWorkItemTask, 0)

	if option == nil {
		return nil, 0, errors.New("nil ListOption")
	}

	opt := options.Find()
	if option.PageNum > 0 && option.PageSize > 0 {
		opt.SetSkip((option.PageNum - 1) * option.PageSize)
		opt.SetLimit(option.PageSize)
	}
	opt.SetSort(bson.D{{"create_time", -1}})

	if len(option.WorkflowName) > 0 {
		query["workflow_name"] = option.WorkflowName
	}
	if len(option.ID) > 0 {
		query["sprint_workitem_ids"] = bson.M{"$regex": option.ID}
	}
	if len(option.Status) > 0 {
		query["status"] = bson.M{"$in": option.Status}
	}

	var (
		err   error
		count int64
	)
	if len(query) == 0 {
		count, err = c.Collection.EstimatedDocumentCount(mongotool.SessionContext(ctx, c.Session))
	} else {
		count, err = c.Collection.CountDocuments(mongotool.SessionContext(ctx, c.Session), query)
	}
	if err != nil {
		return nil, 0, err
	}

	cursor, err := c.Collection.Find(mongotool.SessionContext(ctx, c.Session), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(mongotool.SessionContext(ctx, c.Session), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, count, nil
}

func (c *SprintWorkItemTaskColl) DeleteByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(mongotool.SessionContext(ctx, c.Session), query)
	return err
}
