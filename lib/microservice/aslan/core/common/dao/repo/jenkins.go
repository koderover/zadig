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

type JenkinsIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewJenkinsIntegrationColl() *JenkinsIntegrationColl {
	name := models.JenkinsIntegration{}.TableName()
	coll := &JenkinsIntegrationColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *JenkinsIntegrationColl) GetCollectionName() string {
	return c.coll
}

func (c *JenkinsIntegrationColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *JenkinsIntegrationColl) Create(args *models.JenkinsIntegration) error {
	if args == nil {
		return errors.New("nil jenkins integration args")
	}

	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *JenkinsIntegrationColl) Update(ID string, args *models.JenkinsIntegration) error {
	oldID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oldID}
	change := bson.M{"$set": bson.M{
		"url":        args.URL,
		"username":   args.Username,
		"password":   args.Password,
		"update_by":  args.UpdateBy,
		"updated_at": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *JenkinsIntegrationColl) Delete(ID string) error {
	oldID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oldID}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *JenkinsIntegrationColl) List() ([]*models.JenkinsIntegration, error) {
	resp := make([]*models.JenkinsIntegration, 0)
	query := bson.M{}

	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{"updated_at", -1}})

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
