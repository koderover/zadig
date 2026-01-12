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

type SprintWorkItemActivityColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewSprintWorkItemActivityColl() *SprintWorkItemActivityColl {
	name := models.SprintWorkItemActivity{}.TableName()
	return &SprintWorkItemActivityColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewSprintWorkItemActivityCollWithSession(session mongo.Session) *SprintWorkItemActivityColl {
	name := models.SprintWorkItemActivity{}.TableName()
	return &SprintWorkItemActivityColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *SprintWorkItemActivityColl) GetCollectionName() string {
	return c.coll
}

func (c *SprintWorkItemActivityColl) EnsureIndex(ctx context.Context) error {
	index := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "sprint_workitem_id", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err := c.Indexes().CreateOne(mongotool.SessionContext(ctx, c.Session), index, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *SprintWorkItemActivityColl) Create(ctx *handler.Context, obj *models.SprintWorkItemActivity) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	obj.CreateTime = time.Now().Unix()
	obj.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(mongotool.SessionContext(ctx, c.Session), obj)
	return err
}

func (c *SprintWorkItemActivityColl) Update(ctx *handler.Context, obj *models.SprintWorkItemActivity) error {
	query := bson.M{"_id": obj.ID}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemActivityColl) GetByWorkItemID(ctx *handler.Context, workitemID string) ([]*models.SprintWorkItemActivity, error) {
	query := bson.M{
		"sprint_workitem_id": workitemID,
	}

	cursor, err := c.Collection.Find(mongotool.SessionContext(ctx, c.Session), query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(mongotool.SessionContext(ctx, c.Session))

	var resp []*models.SprintWorkItemActivity
	if err := cursor.All(mongotool.SessionContext(ctx, c.Session), &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *SprintWorkItemActivityColl) DeleteByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(mongotool.SessionContext(ctx, c.Session), query)
	return err
}
