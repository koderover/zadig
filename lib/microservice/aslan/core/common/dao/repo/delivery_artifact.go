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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeliveryArtifactArgs struct {
	ID           string `json:"id"`
	Image        string `json:"image"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	RepoName     string `json:"repo_name"`
	Branch       string `json:"branch"`
	Source       string `json:"source"`
	ImageHash    string `json:"image_hash"`
	ImageTag     string `json:"image_tag"`
	ImageDigest  string `json:"image_digest"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	IsFuzzyQuery bool   `json:"is_fuzzy_query"`
}

type DeliveryArtifactColl struct {
	*mongo.Collection

	coll string
}

func NewDeliveryArtifactColl() *DeliveryArtifactColl {
	name := models.DeliveryArtifact{}.TableName()
	return &DeliveryArtifactColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliveryArtifactColl) GetCollectionName() string {
	return c.coll
}

func (c *DeliveryArtifactColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "type", Value: 1},
			bson.E{Key: "image_tag", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *DeliveryArtifactColl) List(args *DeliveryArtifactArgs) ([]*models.DeliveryArtifact, int, error) {
	if args == nil {
		return nil, 0, errors.New("nil delivery_artifact args")
	}

	resp := make([]*models.DeliveryArtifact, 0)
	query := bson.M{}
	if args.Name != "" {
		if args.IsFuzzyQuery {
			query["name"] = bson.M{"$regex": args.Name}
		} else {
			query["name"] = args.Name
		}
	}

	if args.ImageTag != "" {
		query["image_tag"] = args.ImageTag
	}

	if args.Type != "" {
		query["type"] = args.Type
	}

	if args.Image != "" {
		query["image"] = bson.M{"$regex": args.Image}
	}

	if args.Source != "" {
		query["source"] = args.Source
	}

	// ignore records without image info (image_size, architecture, os, layers, ...)
	// {$or: [{type: {$ne:"image"}}, {type: "image", image_size: { $ne:null}}]}
	query["$or"] = []bson.M{{"type": bson.M{"$ne": "image"}}, {"type": "image", "image_size": bson.M{"$ne": nil}}}

	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, fmt.Errorf("find artifact err:%v", err)
	}
	opt := options.Find().
		SetSort(bson.D{{"created_time", -1}}).
		SetSkip(int64(args.PerPage * (args.Page - 1))).
		SetLimit(int64(args.PerPage))
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, int(count), nil
}

func (c *DeliveryArtifactColl) Get(args *DeliveryArtifactArgs) (*models.DeliveryArtifact, error) {
	if args == nil {
		return nil, errors.New("nil delivery_artifact args")
	}
	resp := new(models.DeliveryArtifact)
	id, err := primitive.ObjectIDFromHex(args.ID)
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

func (c *DeliveryArtifactColl) Insert(args *models.DeliveryArtifact) error {

	if args == nil {
		return errors.New("nil delivery_artifact args")
	}

	result, err := c.InsertOne(context.TODO(), args)
	if err != nil || result == nil {
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		args.ID = oid
	}

	return nil
}

func (c *DeliveryArtifactColl) Update(args *DeliveryArtifactArgs) error {
	query := bson.M{"_id": args.ID}
	if args.ImageHash != "" {
		query["image_hash"] = args.ImageHash
		change := bson.M{"$set": bson.M{
			"image_digest": args.ImageDigest,
		}}
		_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
		return err
	} else {
		query["image_digest"] = args.ImageDigest
		change := bson.M{"$set": bson.M{
			"image_tag": args.ImageTag,
		}}
		_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
		return err
	}
}
