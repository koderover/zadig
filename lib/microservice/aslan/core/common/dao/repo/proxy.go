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

type ProxyArgs struct {
	Type string
}

type ProxyColl struct {
	*mongo.Collection

	coll string
}

func NewProxyColl() *ProxyColl {
	name := models.Proxy{}.TableName()
	return &ProxyColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProxyColl) GetCollectionName() string {
	return c.coll
}

func (c *ProxyColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"address": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *ProxyColl) Find(id string) (*models.Proxy, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	res := &models.Proxy{}
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *ProxyColl) List(args *ProxyArgs) ([]*models.Proxy, error) {
	query := bson.M{}
	if args.Type != "" {
		query["type"] = args.Type
	}

	res := make([]*models.Proxy, 0)
	cursor, err := c.Collection.Find(context.TODO(), query, options.Find().SetSort(bson.D{{"create_time", -1}}))
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, err
}

func (c *ProxyColl) Create(args *models.Proxy) error {
	if args == nil {
		return errors.New("nil proxy info")
	}

	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *ProxyColl) Update(id string, args *models.Proxy) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	if args == nil {
		return errors.New("nil proxy info")
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"type":                     args.Type,
		"address":                  args.Address,
		"port":                     args.Port,
		"need_password":            args.NeedPassword,
		"username":                 args.Username,
		"password":                 args.Password,
		"usage":                    args.Usage,
		"enable_repo_proxy":        args.EnableRepoProxy,
		"enable_application_proxy": args.EnableApplicationProxy,
		"update_by":                args.UpdateBy,
		"update_time":              time.Now().Unix(),
	}}
	_, err = c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProxyColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}
