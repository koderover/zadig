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

type GithubAppColl struct {
	*mongo.Collection

	coll string
}

func NewGithubAppColl() *GithubAppColl {
	name := models.GithubApp{}.TableName()
	coll := &GithubAppColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *GithubAppColl) GetCollectionName() string {
	return c.coll
}

func (c *GithubAppColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *GithubAppColl) Find() ([]*models.GithubApp, error) {
	query := bson.M{}
	ctx := context.Background()
	resp := make([]*models.GithubApp, 0)

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *GithubAppColl) Create(args *models.GithubApp) error {
	if args == nil {
		return errors.New("nil GithubApp")
	}

	args.CreatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *GithubAppColl) Upsert(args *models.GithubApp) error {
	query := bson.M{"_id": args.ID}
	args.CreatedAt = time.Now().Unix()

	change := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *GithubAppColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
