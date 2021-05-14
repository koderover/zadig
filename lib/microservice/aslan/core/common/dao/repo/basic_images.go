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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type BasicImageOpt struct {
	Value     string `bson:"value"`
	Label     string `bson:"label"`
	ImageFrom string `bson:"image_from"`
}

type BasicImageColl struct {
	*mongo.Collection

	coll string
}

func NewBasicImageColl() *BasicImageColl {
	name := models.BasicImage{}.TableName()
	coll := &BasicImageColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *BasicImageColl) GetCollectionName() string {
	return c.coll
}

func (c *BasicImageColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"value": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *BasicImageColl) Find(id string) (*models.BasicImage, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := &models.BasicImage{}
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *BasicImageColl) List(opt *BasicImageOpt) ([]*models.BasicImage, error) {
	query := bson.M{}
	if opt != nil && opt.ImageFrom != "" {
		query["image_from"] = opt.ImageFrom
	}
	if opt != nil && opt.Value != "" {
		query["value"] = opt.Value
	}

	ctx := context.Background()
	resp := make([]*models.BasicImage, 0)

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

func (c *BasicImageColl) Create(args *models.BasicImage) error {
	if args == nil {
		return errors.New("nil BasicImage")
	}

	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *BasicImageColl) Update(id string, args *models.BasicImage) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	if args == nil {
		return errors.New("nil basicImage info")
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"label":       args.Label,
		"value":       args.Value,
		"image_from":  args.ImageFrom,
		"update_by":   args.UpdateBy,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *BasicImageColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *BasicImageColl) InitBasicImageData(basicImageInfos []*models.BasicImage) {
	for _, basicImageInfo := range basicImageInfos {
		ctx := context.Background()
		query := bson.M{"value": basicImageInfo.Value}

		count, _ := c.Collection.CountDocuments(ctx, query)
		if count != 0 {
			continue
		}

		_, err := c.InsertOne(context.TODO(), basicImageInfo)
		if err != nil {
			fmt.Printf("auto create basic image failed, value:%s, err:%v\n", basicImageInfo.Value, err)
		}
	}
}
