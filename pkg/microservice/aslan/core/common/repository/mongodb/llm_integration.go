/*
Copyright 2023 The KodeRover Authors.

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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LLMIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewLLMIntegrationColl() *LLMIntegrationColl {
	name := models.LLMIntegration{}.TableName()
	return &LLMIntegrationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LLMIntegrationColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *LLMIntegrationColl) GetCollectionName() string {
	return c.coll
}

func (c *LLMIntegrationColl) FindByName(ctx context.Context, name string) (*models.LLMIntegration, error) {
	llmProvider := new(models.LLMIntegration)
	query := bson.M{"name": name}
	err := c.FindOne(ctx, query).Decode(llmProvider)
	return llmProvider, err
}

func (c *LLMIntegrationColl) FindByID(ctx context.Context, id string) (*models.LLMIntegration, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.LLMIntegration{}
	err = c.FindOne(ctx, query).Decode(res)

	return res, err
}

func (c *LLMIntegrationColl) Update(ctx context.Context, id string, args *models.LLMIntegration) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	args.ID = oid

	query := bson.M{"_id": args.ID}
	args.UpdateTime = time.Now().Unix()

	change := bson.M{"$set": args}
	_, err = c.UpdateOne(ctx, query, change)
	return err
}

func (c *LLMIntegrationColl) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}

	_, err = c.DeleteOne(ctx, query)
	return err
}

func (c *LLMIntegrationColl) Create(ctx context.Context, args *models.LLMIntegration) error {
	if args == nil {
		return errors.New("nil llm provider args")
	}

	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *LLMIntegrationColl) FindAll(ctx context.Context) ([]*models.LLMIntegration, error) {
	var llmProviders []*models.LLMIntegration
	query := bson.M{}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(ctx, &llmProviders)
	return llmProviders, err
}

func (c *LLMIntegrationColl) Count(ctx context.Context) (int64, error) {
	query := bson.M{}
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		if err == mongo.ErrNoDocuments || err == mongo.ErrNilDocument {
			return 0, nil
		}
		return 0, err
	}

	return count, nil
}
