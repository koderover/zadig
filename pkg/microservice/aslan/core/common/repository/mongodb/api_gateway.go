/*
 * Copyright 2025 The KodeRover Authors.
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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ApiGatewayColl struct {
	*mongo.Collection

	coll string
}

func NewApiGatewayColl() *ApiGatewayColl {
	name := models.ApiGateway{}.TableName()
	coll := &ApiGatewayColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *ApiGatewayColl) GetCollectionName() string {
	return c.coll
}

func (c *ApiGatewayColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "alias", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *ApiGatewayColl) Create(apiGateway *models.ApiGateway) error {
	apiGateway.UpdatedAt = time.Now().Unix()
	_, err := c.Collection.InsertOne(context.TODO(), apiGateway)
	if err != nil {
		log.Error("repository Create err : %v", err)
		return err
	}
	return nil
}

func (c *ApiGatewayColl) GetByID(idHex string) (*models.ApiGateway, error) {
	apiGateway := &models.ApiGateway{}
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.Background(), query).Decode(apiGateway)
	return apiGateway, err
}

func (c *ApiGatewayColl) GetByAlias(alias string) (*models.ApiGateway, error) {
	apiGateway := &models.ApiGateway{}
	query := bson.M{"alias": alias}

	err := c.FindOne(context.Background(), query).Decode(apiGateway)
	return apiGateway, err
}

func (c *ApiGatewayColl) UpdateByID(idHex string, apiGateway *models.ApiGateway) error {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	apiGateway.UpdatedAt = time.Now().Unix()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": apiGateway}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Error("repository UpdateByID err : %v", err)
		return err
	}
	return nil
}

func (c *ApiGatewayColl) DeleteByID(idHex string) error {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.Background(), query)
	return err
}

func (c *ApiGatewayColl) List() ([]*models.ApiGateway, error) {
	query := bson.M{}
	resp := make([]*models.ApiGateway, 0)

	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(context.Background(), &resp)
}

