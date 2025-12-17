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
	"github.com/koderover/zadig/v2/pkg/types"
)

type SprintWorkItemColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewSprintWorkItemColl() *SprintWorkItemColl {
	name := models.SprintWorkItem{}.TableName()
	return &SprintWorkItemColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewSprintWorkItemCollWithSession(session mongo.Session) *SprintWorkItemColl {
	name := models.SprintWorkItem{}.TableName()
	return &SprintWorkItemColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *SprintWorkItemColl) GetCollectionName() string {
	return c.coll
}

func (c *SprintWorkItemColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "sprint_id", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(mongotool.SessionContext(ctx, c.Session), mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *SprintWorkItemColl) Create(ctx *handler.Context, obj *models.SprintWorkItem) (*primitive.ObjectID, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil object")
	}

	obj.CreateTime = time.Now().Unix()
	obj.UpdateTime = time.Now().Unix()
	result, err := c.InsertOne(mongotool.SessionContext(ctx, c.Session), obj)
	if err != nil {
		return nil, err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		return &oid, nil
	}
	return nil, fmt.Errorf("can't convert InsertedID to primitive.ObjectID")
}

func (c *SprintWorkItemColl) Update(ctx *handler.Context, obj *models.SprintWorkItem) error {
	query := bson.M{"_id": obj.ID}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(ctx, query, change)
	return err
}

func (c *SprintWorkItemColl) UpdateOwners(ctx *handler.Context, idStr string, owners []types.UserBriefInfo) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{
		"owners":      owners,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemColl) UpdateTitle(ctx *handler.Context, idStr, title string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{
		"title":       title,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemColl) UpdateDescription(ctx *handler.Context, idStr, description string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{
		"description": description,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemColl) Move(ctx *handler.Context, idStr, sprintID, stageID string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{
		"sprint_id":    sprintID,
		"stage_id":    stageID,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintWorkItemColl) GetByID(ctx *handler.Context, idStr string) (*models.SprintWorkItem, error) {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, err
	}
	query := bson.M{
		"_id": id,
	}

	sprintTemplate := new(models.SprintWorkItem)
	if err := c.Collection.FindOne(mongotool.SessionContext(ctx, c.Session), query).Decode(sprintTemplate); err != nil {
		return nil, err
	}

	return sprintTemplate, nil
}

type ListSprintWorkItemOption struct {
	IDs      []string
	SprintID string
}

func (c *SprintWorkItemColl) List(ctx *handler.Context, listOpt ListSprintWorkItemOption) ([]*models.SprintWorkItem, error) {
	query := bson.M{}

	if len(listOpt.IDs) > 0 {
		objectIDs := make([]primitive.ObjectID, len(listOpt.IDs))
		for i, idStr := range listOpt.IDs {
			id, err := primitive.ObjectIDFromHex(idStr)
			if err != nil {
				return nil, err
			}
			objectIDs[i] = id
		}
		query["_id"] = bson.M{"$in": objectIDs}
	}
	if listOpt.SprintID != "" {
		query["sprint_id"] = listOpt.SprintID
	}

	resp := make([]*models.SprintWorkItem, 0)
	cursor, err := c.Collection.Find(mongotool.SessionContext(ctx, c.Session), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(mongotool.SessionContext(ctx, c.Session), &resp)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *SprintWorkItemColl) DeleteByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	_, err = c.Collection.DeleteOne(mongotool.SessionContext(ctx, c.Session), query)
	return err
}

func (c *SprintWorkItemColl) DeleteByIDs(ctx *handler.Context, ids []string) error {
	objectIDs := make([]primitive.ObjectID, len(ids))
	for i, idStr := range ids {
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			return err
		}
		objectIDs[i] = id
	}

	query := bson.M{
		"_id": bson.M{"$in": objectIDs},
	}

	_, err := c.Collection.DeleteMany(mongotool.SessionContext(ctx, c.Session), query)
	return err
}
