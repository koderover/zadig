/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mongodb

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ReleasePlanColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanColl() *ReleasePlanColl {
	name := models.ReleasePlan{}.TableName()
	return &ReleasePlanColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys:    bson.M{"index": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.M{"name": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"manager": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"status": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"executing_time": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"success_time": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"update_time": 1},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *ReleasePlanColl) Create(args *models.ReleasePlan) (string, error) {
	if args == nil {
		return "", errors.New("nil ReleasePlan")
	}

	res, err := c.InsertOne(context.Background(), args)
	return res.InsertedID.(primitive.ObjectID).Hex(), err
}

func (c *ReleasePlanColl) GetByID(ctx context.Context, idString string) (*models.ReleasePlan, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": id}
	result := new(models.ReleasePlan)
	err = c.FindOne(ctx, query).Decode(result)
	return result, err
}

func (c *ReleasePlanColl) UpdateByID(ctx context.Context, idString string, args *models.ReleasePlan) error {
	if args == nil {
		return errors.New("nil ReleasePlan")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	query := bson.M{"_id": id}
	change := bson.M{"$set": args}
	_, err = c.UpdateOne(ctx, query, change)
	return err
}

func (c *ReleasePlanColl) DeleteByID(ctx context.Context, idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	_, err = c.DeleteOne(ctx, query)
	return err
}

type SortReleasePlanBy string

const (
	SortReleasePlanByIndex      SortReleasePlanBy = "index"
	SortReleasePlanByUpdateTime SortReleasePlanBy = "update_time"
)

type ListReleasePlanOption struct {
	PageNum          int64
	PageSize         int64
	Name             string
	Manager          string
	SuccessTimeStart int64
	SuccessTimeEnd   int64
	UpdateTimeStart  int64
	UpdateTimeEnd    int64
	IsSort           bool
	SortBy           SortReleasePlanBy
	ExcludedFields   []string
	Status           config.ReleasePlanStatus
}

func (c *ReleasePlanColl) ListByOptions(opt *ListReleasePlanOption) ([]*models.ReleasePlan, int64, error) {
	if opt == nil {
		return nil, 0, errors.New("nil ListOption")
	}

	query := bson.M{}

	var resp []*models.ReleasePlan
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		if opt.SortBy == SortReleasePlanByIndex {
			opts.SetSort(bson.D{{"index", -1}})
		} else if opt.SortBy == SortReleasePlanByUpdateTime {
			opts.SetSort(bson.D{{"update_time", -1}})
		} else {
			opts.SetSort(bson.D{{"index", -1}})
		}
	}

	if opt.PageNum > 0 && opt.PageSize > 0 {
		opts.SetSkip((opt.PageNum - 1) * opt.PageSize)
		opts.SetLimit(opt.PageSize)
	}
	if opt.Name != "" {
		query["name"] = bson.M{"$regex": fmt.Sprintf(".*%s.*", opt.Name), "$options": "i"}
	}
	if opt.Manager != "" {
		query["manager"] = bson.M{"$regex": fmt.Sprintf(".*%s.*", opt.Manager), "$options": "i"}
	}
	if opt.SuccessTimeStart > 0 && opt.SuccessTimeEnd > 0 {
		query["success_time"] = bson.M{"$gte": opt.SuccessTimeStart, "$lte": opt.SuccessTimeEnd}
	}
	if opt.UpdateTimeStart > 0 && opt.UpdateTimeEnd > 0 {
		query["update_time"] = bson.M{"$gte": opt.UpdateTimeStart, "$lte": opt.UpdateTimeEnd}
	}
	if opt.Status != "" {
		query["status"] = opt.Status
	}
	if len(opt.ExcludedFields) > 0 {
		projection := bson.M{}
		for _, field := range opt.ExcludedFields {
			projection[field] = 0
		}
		opts.SetProjection(projection)
	}

	var (
		err   error
		count int64
	)
	if len(query) == 0 {
		count, err = c.Collection.EstimatedDocumentCount(ctx)
	} else {
		count, err = c.Collection.CountDocuments(ctx, query)
	}
	if err != nil {
		return nil, 0, err
	}

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

// ListFinishedReleasePlan list all finished release plans in the given time period
func (c *ReleasePlanColl) ListFinishedReleasePlan(startTime, endTime int64) ([]*models.ReleasePlan, error) {
	query := bson.M{}

	timeQuery := bson.M{}
	if startTime > 0 {
		timeQuery["$gte"] = startTime
	}
	if endTime > 0 {
		timeQuery["$lt"] = endTime
	}

	if startTime > 0 || endTime > 0 {
		query["executing_time"] = timeQuery
	}

	query["success_time"] = bson.M{"$ne": 0}

	cursor, err := c.Collection.Find(context.TODO(), query, options.Find())
	if err != nil {
		return nil, err
	}

	resp := make([]*models.ReleasePlan, 0)
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *ReleasePlanColl) ListByCursor() (*mongo.Cursor, error) {
	query := bson.M{}
	return c.Collection.Find(context.TODO(), query)
}
