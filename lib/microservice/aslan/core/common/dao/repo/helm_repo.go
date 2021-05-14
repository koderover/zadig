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

type HelmRepoColl struct {
	*mongo.Collection

	coll string
}

func NewHelmRepoColl() *HelmRepoColl {
	name := models.HelmRepo{}.TableName()
	coll := &HelmRepoColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *HelmRepoColl) GetCollectionName() string {
	return c.coll
}

func (c *HelmRepoColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *HelmRepoColl) Create(args *models.HelmRepo) error {
	if args == nil {
		return errors.New("nil helm repo args")
	}

	args.CreatedAt = time.Now().Unix()
	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *HelmRepoColl) Update(id string, args *models.HelmRepo) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"repo_name":  args.RepoName,
		"url":        args.URL,
		"username":   args.Username,
		"password":   args.Password,
		"update_by":  args.UpdateBy,
		"updated_at": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *HelmRepoColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *HelmRepoColl) List() ([]*models.HelmRepo, error) {
	resp := make([]*models.HelmRepo, 0)
	query := bson.M{}

	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{"created_at", -1}})

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
