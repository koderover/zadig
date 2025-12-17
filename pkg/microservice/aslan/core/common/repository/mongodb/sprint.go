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

type SprintQueryOption struct {
	ID          string
	Name        string
	ProjectName string
}

type SprintListOption struct {
	CreatedBy string
}

type SprintColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewSprintColl() *SprintColl {
	name := models.Sprint{}.TableName()
	return &SprintColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewSprintCollWithSession(session mongo.Session) *SprintColl {
	name := models.Sprint{}.TableName()
	return &SprintColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *SprintColl) GetCollectionName() string {
	return c.coll
}

func (c *SprintColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "key", Value: 1},
				bson.E{Key: "name", Value: 1},
				bson.E{Key: "is_archived", Value: 1},
				bson.E{Key: "key_initials", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("project_name_filter_create_time_idx"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "is_archived", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("project_name_create_time_idx"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "_id", Value: 1},
				bson.E{Key: "is_archived", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("_id_is_archived_idx"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("create_time_idx"),
		},
	}

	_, err := c.Indexes().CreateMany(mongotool.SessionContext(ctx, c.Session), mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *SprintColl) Create(ctx *handler.Context, obj *models.Sprint) (*primitive.ObjectID, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil object")
	}

	obj.CreateTime = time.Now().Unix()
	obj.UpdateTime = time.Now().Unix()
	obj.CreatedBy = ctx.GenUserBriefInfo()
	obj.UpdatedBy = ctx.GenUserBriefInfo()
	result, err := c.InsertOne(mongotool.SessionContext(ctx, c.Session), obj)
	if err != nil {
		return nil, err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		return &oid, nil
	}
	return nil, fmt.Errorf("can't convert InsertedID to primitive.ObjectID")
}

func (c *SprintColl) Update(ctx *handler.Context, obj *models.Sprint) error {
	query := bson.M{"_id": obj.ID, "is_archived": false}
	change := bson.M{"$set": obj}
	obj.UpdateTime = time.Now().Unix()
	_, err := c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintColl) BulkAddStage(ctx *handler.Context, sprintIDs []primitive.ObjectID, stage *models.SprintStageTemplate) error {
	var bulkOps []mongo.WriteModel
	for _, sprintID := range sprintIDs {
		query := bson.M{"_id": sprintID, "is_archived": false}
		update := bson.M{
			"$push": bson.M{
				"stages": stage,
			},
			"$set": bson.M{
				"update_time": time.Now().Unix(),
				"updated_by":  ctx.GenUserBriefInfo(),
			},
		}

		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(query).SetUpdate(update))
	}

	if len(bulkOps) > 0 {
		_, err := c.Collection.BulkWrite(ctx, bulkOps)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SprintColl) BulkDeleteStage(ctx *handler.Context, sprintIDs []primitive.ObjectID, stageID string) error {
	var bulkOps []mongo.WriteModel
	for _, sprintID := range sprintIDs {
		query := bson.M{"_id": sprintID, "is_archived": false}
		update := bson.M{
			"$pull": bson.M{
				"stages": bson.M{"id": stageID},
			},
			"$set": bson.M{
				"update_time": time.Now().Unix(),
				"updated_by":  ctx.GenUserBriefInfo(),
			},
		}

		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(query).SetUpdate(update))
	}

	if len(bulkOps) > 0 {
		_, err := c.Collection.BulkWrite(ctx, bulkOps)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SprintColl) BulkUpdateStageName(ctx *handler.Context, sprintIDs []primitive.ObjectID, stageID, stageName string) error {
	var bulkOps []mongo.WriteModel
	for _, sprintID := range sprintIDs {
		query := bson.M{"_id": sprintID, "is_archived": false}
		update := bson.M{
			"$set": bson.M{
				"stages.$[stage].name": stageName,
				"update_time":          time.Now().Unix(),
				"updated_by":           ctx.GenUserBriefInfo(),
			},
		}
		arrayFilters := options.ArrayFilters{
			Filters: []interface{}{bson.M{"stage.id": stageID}},
		}

		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(query).SetUpdate(update).SetArrayFilters(arrayFilters))
	}

	if len(bulkOps) > 0 {
		_, err := c.Collection.BulkWrite(mongotool.SessionContext(ctx, c.Session), bulkOps)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SprintColl) BulkUpdateStageWorkflows(ctx *handler.Context, stageID string, workflowsMap map[string][]*models.SprintWorkflow) error {
	var bulkOps []mongo.WriteModel
	for sprintIDStr, workflows := range workflowsMap {
		sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
		if err != nil {
			return err
		}

		query := bson.M{"_id": sprintID, "is_archived": false}
		update := bson.M{
			"$set": bson.M{
				"stages.$[stage].workflows": workflows,
				"update_time":               time.Now().Unix(),
				"updated_by":                ctx.GenUserBriefInfo(),
			},
		}
		arrayFilters := options.ArrayFilters{
			Filters: []interface{}{bson.M{"stage.id": stageID}},
		}

		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(query).SetUpdate(update).SetArrayFilters(arrayFilters))
	}

	if len(bulkOps) > 0 {
		_, err := c.Collection.BulkWrite(mongotool.SessionContext(ctx, c.Session), bulkOps)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SprintColl) GetByID(ctx *handler.Context, idStr string) (*models.Sprint, error) {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, err
	}
	query := bson.M{
		"_id": id,
	}

	sprintTemplate := new(models.Sprint)
	if err := c.Collection.FindOne(mongotool.SessionContext(ctx, c.Session), query).Decode(sprintTemplate); err != nil {
		return nil, err
	}

	return sprintTemplate, nil
}

func (c *SprintColl) Find(ctx *handler.Context, opt *SprintQueryOption) (*models.Sprint, error) {
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
	if len(opt.ProjectName) > 0 {
		query["project_name"] = opt.ProjectName
	}
	if len(opt.Name) > 0 {
		query["name"] = opt.Name
	}
	resp := new(models.Sprint)
	err := c.Collection.FindOne(mongotool.SessionContext(ctx, c.Session), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type ListSprintOption struct {
	PageNum        int64
	PageSize       int64
	ProjectName    string
	Name           string
	Key            string
	Filter         string
	TemplateID     string
	IsArchived     bool
	ExcludedFields []string
}

func (c *SprintColl) List(ctx *handler.Context, opt *ListSprintOption) ([]*models.Sprint, int64, error) {
	if opt == nil {
		return nil, 0, errors.New("nil ListOption")
	}

	findOption := bson.M{}
	if len(opt.ProjectName) > 0 {
		findOption["project_name"] = opt.ProjectName
	}
	if len(opt.Name) > 0 {
		findOption["name"] = opt.Name
	}
	if len(opt.Key) > 0 {
		findOption["key"] = opt.Key
	}
	findOption["is_archived"] = opt.IsArchived

	finalSearchCondition := []bson.M{
		findOption,
	}

	if opt.Filter != "" {
		finalSearchCondition = append(finalSearchCondition, bson.M{
			"$or": bson.A{
				bson.M{"name": bson.M{"$regex": opt.Filter, "$options": "i"}},
				bson.M{"key": bson.M{"$regex": opt.Filter, "$options": "i"}},
				bson.M{"key_initials": bson.M{"$regex": opt.Filter, "$options": "i"}},
			},
		})
	}

	filter := bson.M{
		"$and": finalSearchCondition,
	}

	pipeline := []bson.M{
		{
			"$match": filter,
		},
		{
			"$sort": bson.M{"create_time": -1}, // order by create_time desc
		},
	}

	if opt.PageNum > 0 && opt.PageSize > 0 {
		pipeline = append(pipeline, bson.M{
			"$facet": bson.M{
				"data": []bson.M{
					{
						"$skip": (opt.PageNum - 1) * opt.PageSize,
					},
					{
						"$limit": opt.PageSize,
					},
				},
				"total": []bson.M{
					{
						"$count": "total",
					},
				},
			},
		})
	} else {
		pipeline = append(pipeline, bson.M{
			"$facet": bson.M{
				"data": []bson.M{
					{"$skip": 0},
				},
				"total": []bson.M{
					{
						"$count": "total",
					},
				},
			},
		})
	}

	result := make([]struct {
		Data  []*models.Sprint `bson:"data"`
		Total []struct {
			Total int64 `bson:"total"`
		} `bson:"total"`
	}, 0)

	cursor, err := c.Collection.Aggregate(mongotool.SessionContext(ctx, c.Session), pipeline)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(mongotool.SessionContext(ctx, c.Session), &result)
	if err != nil {
		return nil, 0, err
	}

	if len(result) == 0 {
		return nil, 0, nil
	}

	var res []*models.Sprint
	if len(result[0].Data) > 0 {
		res = result[0].Data
	}

	total := int64(0)
	if len(result[0].Total) > 0 {
		total = result[0].Total[0].Total
	}

	return res, total, nil
}

func (c *SprintColl) DeleteByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(mongotool.SessionContext(ctx, c.Session), query)
	return err
}

func (c *SprintColl) ArchiveByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id, "is_archived": false}
	change := bson.M{"$set": bson.M{
		"is_archived": true,
		"update_time": time.Now().Unix(),
		"updated_by":  ctx.GenUserBriefInfo(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintColl) ActivateArchivedByID(ctx *handler.Context, idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id, "is_archived": true}
	change := bson.M{"$set": bson.M{
		"is_archived": false,
		"update_time": time.Now().Unix(),
		"updated_by":  ctx.GenUserBriefInfo(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintColl) UpdateName(ctx *handler.Context, idStr string, obj *models.Sprint) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id, "is_archived": false}
	change := bson.M{"$set": bson.M{
		"name":         obj.Name,
		"key":          obj.Key,
		"key_initials": obj.KeyInitials,
		"update_time":  time.Now().Unix(),
		"updated_by":   ctx.GenUserBriefInfo(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

// Use it in CAUTION, may cause data inconsistency
func (c *SprintColl) UpdateAllStages(ctx *handler.Context, idStr string, obj []*models.SprintStage) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id, "is_archived": false}
	change := bson.M{"$set": bson.M{
		"stages":      obj,
		"update_time": time.Now().Unix(),
		"updated_by":  ctx.GenUserBriefInfo(),
	}}
	_, err = c.UpdateOne(mongotool.SessionContext(ctx, c.Session), query, change)
	return err
}

func (c *SprintColl) UpdateStageName(ctx *handler.Context, sprintIDStr string, stageID string, stageName string) error {
	sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": sprintID, "is_archived": false}

	update := bson.M{
		"$set": bson.M{
			"stages.$[stage].name": stageName,
			"update_time":          time.Now().Unix(),
			"updated_by":           ctx.GenUserBriefInfo(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"stage.id": stageID}},
	}

	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	res, err := c.Collection.UpdateOne(ctx, query, update, &updateOptions)
	if res.ModifiedCount == 0 {
		return fmt.Errorf("UpdateStageName: no document updated, sprintID: %s, stageID: %s", sprintIDStr, stageID)
	}
	return err
}

func (c *SprintColl) AddStageWorkItemID(ctx *handler.Context, sprintIDStr string, stageID string, workItemID string) error {
	sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": sprintID, "is_archived": false}

	update := bson.M{
		"$push": bson.M{
			"stages.$[stage].workitem_ids": workItemID,
		},
		"$set": bson.M{
			"update_time": time.Now().Unix(),
			"updated_by":  ctx.GenUserBriefInfo(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"stage.id": stageID}},
	}

	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	res, err := c.Collection.UpdateOne(ctx, query, update, &updateOptions)
	if res.ModifiedCount == 0 {
		return fmt.Errorf("AddStageWorkIemID: no document updated, sprintID: %s, stageID: %s, workItemID: %s", sprintIDStr, stageID, workItemID)
	}

	return err
}

func (c *SprintColl) DeleteStageWorkItemID(ctx *handler.Context, sprintIDStr string, stageID string, workItemID string) error {
	sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": sprintID, "is_archived": false}

	update := bson.M{
		"$pull": bson.M{
			"stages.$[stage].workitem_ids": workItemID,
		},
		"$set": bson.M{
			"update_time": time.Now().Unix(),
			"updated_by":  ctx.GenUserBriefInfo(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"stage.id": stageID}},
	}

	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	res, err := c.Collection.UpdateOne(ctx, query, update, &updateOptions)
	if res.ModifiedCount == 0 {
		return fmt.Errorf("DeleteStageWorkItemID: no document updated, sprintID: %s, stageID: %s, workItemID: %s", sprintIDStr, stageID, workItemID)
	}

	return err
}

func (c *SprintColl) UpdateStageWorkItemIDs(ctx *handler.Context, sprintIDStr string, stageID string, workItemIDs []string) error {
	sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": sprintID, "is_archived": false}

	update := bson.M{
		"$set": bson.M{
			"stages.$[stage].workitem_ids": workItemIDs,
			"update_time":                  time.Now().Unix(),
			"updated_by":                   ctx.GenUserBriefInfo(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"stage.id": stageID}},
	}

	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	res, err := c.Collection.UpdateOne(ctx, query, update, &updateOptions)
	if res.ModifiedCount == 0 {
		return fmt.Errorf("UpdateStageWorkItemIDs: no document updated, sprintID: %s, stageID: %s", sprintIDStr, stageID)
	}

	return err
}

func (c *SprintColl) UpdateStageWorkflows(ctx *handler.Context, sprintIDStr string, stageID string, workflows []*models.SprintWorkflow) error {
	sprintID, err := primitive.ObjectIDFromHex(sprintIDStr)
	if err != nil {
		return err
	}

	query := bson.M{"_id": sprintID, "is_archived": false}

	update := bson.M{
		"$set": bson.M{
			"stages.$[stage].workflows": workflows,
			"update_time":               time.Now().Unix(),
			"updated_by":                ctx.GenUserBriefInfo(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"stage.id": stageID}},
	}

	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	res, err := c.Collection.UpdateOne(ctx, query, update, &updateOptions)
	if res.ModifiedCount == 0 {
		return fmt.Errorf("UpdateStageWorkflows: no document updated, sprintID: %s, stageID: %s", sprintIDStr, stageID)
	}

	return err
}
