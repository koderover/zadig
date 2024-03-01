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

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ReleasePlanLogColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanLogColl() *ReleasePlanLogColl {
	name := models.ReleasePlanLog{}.TableName()
	return &ReleasePlanLogColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanLogColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanLogColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *ReleasePlanLogColl) Create(args *models.ReleasePlanLog) error {
	if args == nil {
		return errors.New("nil ReleasePlanLog")
	}

	_, err := c.InsertOne(context.Background(), args)
	return err
}

type ListReleasePlanLogOption struct {
	PlanID string
	IsSort bool
}

func (c *ReleasePlanLogColl) ListByOptions(opt *ListReleasePlanLogOption) ([]*models.ReleasePlanLog, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	query := bson.M{}

	var resp []*models.ReleasePlanLog
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"create_time", -1}})
	}
	if opt.PlanID != "" {
		query["plan_id"] = opt.PlanID
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
