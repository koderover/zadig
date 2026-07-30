/*
Copyright 2026 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type AgentIntegrationColl struct {
	*mongo.Collection
	coll string
}

func NewAgentIntegrationColl() *AgentIntegrationColl {
	name := models.AgentIntegration{}.TableName()
	return &AgentIntegrationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *AgentIntegrationColl) GetCollectionName() string { return c.coll }

func (c *AgentIntegrationColl) EnsureIndex(ctx context.Context) error {
	// drop the legacy global unique index on name; uniqueness is now per project
	_, _ = c.Indexes().DropOne(ctx, "name_1")
	_, err := c.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "project_name", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (c *AgentIntegrationColl) Create(ctx context.Context, integration *models.AgentIntegration) error {
	if integration == nil {
		return errors.New("agent integration is nil")
	}
	if integration.ID.IsZero() {
		integration.ID = primitive.NewObjectID()
	}
	integration.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(ctx, integration)
	return err
}

func (c *AgentIntegrationColl) Update(ctx context.Context, id string, integration *models.AgentIntegration) error {
	if integration == nil {
		return errors.New("agent integration is nil")
	}
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	integration.ID = oid
	integration.UpdateTime = time.Now().Unix()
	result, err := c.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": integration})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (c *AgentIntegrationColl) FindByID(ctx context.Context, id string) (*models.AgentIntegration, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	result := new(models.AgentIntegration)
	return result, c.FindOne(ctx, bson.M{"_id": oid}).Decode(result)
}

func (c *AgentIntegrationColl) ListByProject(ctx context.Context, projectName string) ([]*models.AgentIntegration, error) {
	result := make([]*models.AgentIntegration, 0)
	cursor, err := c.Find(ctx, bson.M{"project_name": projectName}, options.Find().SetSort(bson.D{{Key: "update_time", Value: -1}}))
	if err != nil {
		return nil, err
	}
	return result, cursor.All(ctx, &result)
}

func (c *AgentIntegrationColl) ListAll(ctx context.Context) ([]*models.AgentIntegration, error) {
	result := make([]*models.AgentIntegration, 0)
	cursor, err := c.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "update_time", Value: -1}}))
	if err != nil {
		return nil, err
	}
	return result, cursor.All(ctx, &result)
}

func (c *AgentIntegrationColl) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	result, err := c.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
