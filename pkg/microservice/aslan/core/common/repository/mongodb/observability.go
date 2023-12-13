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
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ObservabilityColl struct {
	*mongo.Collection

	coll string
}

func NewObservabilityColl() *ObservabilityColl {
	name := models.Observability{}.TableName()
	return &ObservabilityColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ObservabilityColl) GetCollectionName() string {
	return c.coll
}

func (c *ObservabilityColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *ObservabilityColl) Create(ctx context.Context, args *models.Observability) error {
	if args == nil {
		return errors.New("observability is nil")
	}
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *ObservabilityColl) Update(ctx context.Context, idString string, args *models.Observability) error {
	if args == nil {
		return errors.New("observability is nil")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	args.UpdateTime = time.Now().Unix()

	query := bson.M{"_id": id}
	change := bson.M{"$set": args}
	_, err = c.UpdateOne(ctx, query, change)
	return err
}

func (c *ObservabilityColl) List(ctx context.Context, _type string) ([]*models.Observability, error) {
	resp := make([]*models.Observability, 0)
	query := bson.M{}
	if _type != "" {
		query["type"] = _type
	}
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(ctx, &resp)
}

func (c *ObservabilityColl) GetByID(ctx context.Context, idString string) (*models.Observability, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": id}
	resp := new(models.Observability)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *ObservabilityColl) DeleteByID(ctx context.Context, idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	_, err = c.DeleteOne(ctx, query)
	return err
}
