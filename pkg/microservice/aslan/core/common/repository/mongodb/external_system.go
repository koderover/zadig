/*
Copyright 2021 The KodeRover Authors.

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

type ExternalSystemColl struct {
	*mongo.Collection

	coll string
}

func NewExternalSystemColl() *ExternalSystemColl {
	name := models.ExternalSystem{}.TableName()
	return &ExternalSystemColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ExternalSystemColl) GetCollectionName() string {
	return c.coll
}

func (c *ExternalSystemColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *ExternalSystemColl) Create(args *models.ExternalSystem) error {
	if args == nil {
		return errors.New("external system is nil")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *ExternalSystemColl) List(pageNum, pageSize int64) ([]*models.ExternalSystem, int64, error) {
	query := bson.M{}
	resp := make([]*models.ExternalSystem, 0)
	ctx := context.Background()

	opt := options.Find().
		SetSkip((pageNum - 1) * pageSize).
		SetLimit(pageSize)

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

func (c *ExternalSystemColl) GetByID(idstring string) (*models.ExternalSystem, error) {
	resp := new(models.ExternalSystem)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ExternalSystemColl) Update(idString string, obj *models.ExternalSystem) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *ExternalSystemColl) DeleteByID(idstring string) error {
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
