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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type PrivateKeyArgs struct {
	Name string
}

type PrivateKeyColl struct {
	*mongo.Collection

	coll string
}

func NewPrivateKeyColl() *PrivateKeyColl {
	name := models.PrivateKey{}.TableName()
	return &PrivateKeyColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PrivateKeyColl) GetCollectionName() string {
	return c.coll
}

func (c *PrivateKeyColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"label": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

type FindPrivateKeyOption struct {
	ID      string
	Address string
}

func (c *PrivateKeyColl) Find(option FindPrivateKeyOption) (*models.PrivateKey, error) {
	privateKey := new(models.PrivateKey)
	query := bson.M{}
	if option.ID != "" {
		oid, err := primitive.ObjectIDFromHex(option.ID)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	if option.Address != "" {
		query["ip"] = option.Address
	}

	err := c.FindOne(context.TODO(), query).Decode(privateKey)
	return privateKey, err
}

func (c *PrivateKeyColl) List(args *PrivateKeyArgs) ([]*models.PrivateKey, error) {
	query := bson.M{}
	if args.Name != "" {
		query["name"] = args.Name
	}
	resp := make([]*models.PrivateKey, 0)
	ctx := context.Background()

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

func (c *PrivateKeyColl) Create(args *models.PrivateKey) error {
	if args == nil {
		return errors.New("nil PrivateKey info")
	}

	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *PrivateKeyColl) Update(id string, args *models.PrivateKey) error {
	if args == nil {
		return errors.New("nil PrivateKey info")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"name":        args.Name,
		"user_name":   args.UserName,
		"ip":          args.IP,
		"label":       args.Label,
		"is_prod":     args.IsProd,
		"private_key": args.PrivateKey,
		"update_by":   args.UpdateBy,
		"update_time": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *PrivateKeyColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *PrivateKeyColl) ListHostIPByIDs(ids []string) ([]*models.PrivateKey, error) {
	query := bson.M{}
	resp := make([]*models.PrivateKey, 0)
	ctx := context.Background()

	var oids []primitive.ObjectID
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		oids = append(oids, oid)
	}
	query["_id"] = bson.M{"$in": oids}
	opt := options.Find()
	selector := bson.D{
		{"ip", 1},
	}
	opt.SetProjection(selector)
	cursor, err := c.Collection.Find(ctx, query, opt)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}
