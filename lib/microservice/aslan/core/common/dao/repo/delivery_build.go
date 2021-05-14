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

package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeliveryBuildArgs struct {
	ID          string `json:"id"`
	ReleaseID   string `json:"releaseId"`
	ServiceName string `json:"serviceName"`
}

type DeliveryBuildColl struct {
	*mongo.Collection

	coll string
}

func NewDeliveryBuildColl() *DeliveryBuildColl {
	name := models.DeliveryBuild{}.TableName()
	return &DeliveryBuildColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliveryBuildColl) GetCollectionName() string {
	return c.coll
}

func (c *DeliveryBuildColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "release_id", Value: 1},
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "release_id", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *DeliveryBuildColl) Find(args *DeliveryBuildArgs) ([]*models.DeliveryBuild, error) {
	if args == nil {
		return nil, errors.New("nil delivery_build args")
	}
	resp := make([]*models.DeliveryBuild, 0)
	releaseID, err := primitive.ObjectIDFromHex(args.ReleaseID)
	if err != nil {
		return nil, err
	}
	query := bson.M{"release_id": releaseID, "deleted_at": 0}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DeliveryBuildColl) Delete(releaseID string) error {
	oid, err := primitive.ObjectIDFromHex(releaseID)
	if err != nil {
		return err
	}
	query := bson.M{"release_id": oid, "deleted_at": 0}

	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryBuildColl) Insert(args *models.DeliveryBuild) error {
	if args == nil {
		return errors.New("nil delivery_build args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}
