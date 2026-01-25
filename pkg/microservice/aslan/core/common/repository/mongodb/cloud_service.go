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
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CloudServiceColl struct {
	*mongo.Collection
}

type CloudServiceCollFindOption struct {
	Id   string
	Name string
}

func NewCloudServiceColl() *CloudServiceColl {
	coll := &CloudServiceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(models.CloudService{}.TableName())}
	return coll
}

func (c *CloudServiceColl) GetCollectionName() string {
	return models.CloudService{}.TableName()
}

func (c *CloudServiceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *CloudServiceColl) List(ctx context.Context) ([]*models.CloudService, error) {
	resp := make([]*models.CloudService, 0)
	query := bson.M{}

	opts := options.Find().SetSort(bson.D{{"created_at", -1}})

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *CloudServiceColl) Find(ctx context.Context, opt *CloudServiceCollFindOption) (*models.CloudService, error) {
	query := bson.M{}
	if len(opt.Id) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.Id)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	if opt.Name != "" {
		query["name"] = opt.Name
	}

	resp := &models.CloudService{}
	err := c.FindOne(context.Background(), query).Decode(resp)
	return resp, err
}

func (c *CloudServiceColl) Create(ctx context.Context, args *models.CloudService) error {
	if args == nil {
		return errors.New("nil cloud service args")
	}

	args.CreatedAt = time.Now().Unix()
	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *CloudServiceColl) Update(ctx context.Context, id string, args *models.CloudService) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	args.UpdatedAt = time.Now().Unix()
	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"name":              args.Name,
		"type":              args.Type,
		"region":            args.Region,
		"access_key_id":     args.AccessKeyId,
		"access_key_secret": args.AccessKeySecret,
		"update_by":         args.UpdateBy,
		"updated_at":        time.Now().Unix(),
	}}

	_, err = c.UpdateOne(ctx, query, change, options.Update().SetUpsert(true))
	return err
}

func (c *CloudServiceColl) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(ctx, query)

	return err
}

func (c *CloudServiceColl) FindDefault(ctx context.Context, typ setting.CloudServiceType) (*models.CloudService, error) {
	query := bson.M{"type": typ}
	resp := &models.CloudService{}
	err := c.FindOne(ctx, query).Decode(resp)
	return resp, err
}
