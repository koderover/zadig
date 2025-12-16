/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LabelColl struct {
	*mongo.Collection
	coll string
}

func NewLabelColl() *LabelColl {
	name := models.Label{}.TableName()
	return &LabelColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LabelColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "key", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *LabelColl) Create(args *models.Label) error {
	if args == nil {
		return fmt.Errorf("given label is nil")
	}

	args.CreatedAt = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type LabelListOption struct {
	Keys []string
}

func (c *LabelColl) List(opt *LabelListOption) ([]*models.Label, error) {
	var labels []*models.Label

	query := bson.M{}

	if opt.Keys != nil && len(opt.Keys) > 0 {
		query["key"] = bson.M{
			"$in": opt.Keys,
		}
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &labels)
	if err != nil {
		return nil, err
	}

	return labels, err
}

func (c *LabelColl) Update(id string, args *models.Label) error {
	labelID, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		return err
	}
	_, err = c.UpdateOne(context.TODO(),
		bson.M{"_id": labelID}, bson.M{"$set": bson.M{
			"key":         args.Key,
			"description": args.Description,
			"updated_at":  args.UpdatedAt,
		}},
	)

	return err
}

func (c *LabelColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *LabelColl) GetByID(id string) (*models.Label, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.Label{}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}
