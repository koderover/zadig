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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type CronjobDeleteOption struct {
	IdList     []string
	ParentName string
	ParentType string
}

type ListCronjobParam struct {
	ParentName string
	ParentType string
}

type CronjobColl struct {
	*mongo.Collection

	coll string
}

func NewCronjobColl() *CronjobColl {
	name := models.Cronjob{}.TableName()
	return &CronjobColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *CronjobColl) GetCollectionName() string {
	return c.coll
}

func (c *CronjobColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *CronjobColl) Delete(param *CronjobDeleteOption) error {
	query := bson.M{}

	if len(param.IdList) > 0 {
		var oids []primitive.ObjectID
		for _, id := range param.IdList {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}

	if param.ParentType != "" && param.ParentName != "" {
		query["name"] = param.ParentName
		query["type"] = param.ParentType
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *CronjobColl) List(param *ListCronjobParam) ([]*models.Cronjob, error) {
	resp := make([]*models.Cronjob, 0)
	query := bson.M{}
	if param.ParentType != "" {
		query["type"] = param.ParentType
	}
	if param.ParentName != "" {
		query["name"] = param.ParentName
	}

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

func (c *CronjobColl) Create(args *models.Cronjob) error {
	if args == nil {
		return errors.New("nil cron job args")
	}

	result, err := c.InsertOne(context.TODO(), args)
	if err != nil {
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		args.ID = oid
	}

	return nil
}

func (c *CronjobColl) Update(job *models.Cronjob) error {
	if job == nil {
		return errors.New("nil cron job args")
	}

	query := bson.M{"_id": job.ID}
	change := bson.M{"$set": job}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *CronjobColl) ListActiveJob() ([]*models.Cronjob, error) {
	resp := make([]*models.Cronjob, 0)
	query := bson.M{}
	query["enabled"] = true
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	return resp, err
}
