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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type SonarIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewSonarIntegrationColl() *SonarIntegrationColl {
	name := models.SonarIntegration{}.TableName()
	return &SonarIntegrationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *SonarIntegrationColl) GetCollectionName() string {
	return c.coll
}

func (c *SonarIntegrationColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *SonarIntegrationColl) Create(ctx context.Context, args *models.SonarIntegration) error {
	if args == nil {
		return errors.New("sonar integration is nil")
	}

	_, err := c.InsertOne(ctx, args)

	return err
}

func (c *SonarIntegrationColl) List(ctx context.Context, pageNum, pageSize int64) ([]*models.SonarIntegration, int64, error) {
	query := bson.M{}
	resp := make([]*models.SonarIntegration, 0)

	opt := options.Find()

	if pageNum != 0 && pageSize != 0 {
		opt = opt.
			SetSkip((pageNum - 1) * pageSize).
			SetLimit(pageSize)
	}

	cursor, err := c.Collection.Find(ctx, query, opt)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, 0, err
	}
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

func (c *SonarIntegrationColl) GetByID(ctx context.Context, idstring string) (*models.SonarIntegration, error) {
	resp := new(models.SonarIntegration)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(ctx, query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *SonarIntegrationColl) Update(ctx context.Context, idString string, obj *models.SonarIntegration) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(ctx, filter, update)
	return err
}

func (c *SonarIntegrationColl) DeleteByID(ctx context.Context, idstring string) error {
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(ctx, query)
	return err
}
