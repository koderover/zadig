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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type FavoriteArgs struct {
	UserID      int
	ProductName string
	Name        string
	Type        string
}

type FavoriteColl struct {
	*mongo.Collection

	coll string
}

func NewFavoriteColl() *FavoriteColl {
	name := models.Favorite{}.TableName()
	return &FavoriteColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *FavoriteColl) GetCollectionName() string {
	return c.coll
}

func (c *FavoriteColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "user_id", Value: 1},
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "type", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *FavoriteColl) Create(args *models.Favorite) error {
	if args == nil {
		return errors.New("nil Favorite args")
	}

	args.CreateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *FavoriteColl) List(args *FavoriteArgs) ([]*models.Favorite, error) {
	query := bson.M{"user_id": args.UserID}
	if args.ProductName != "" {
		query["product_name"] = args.ProductName
	}
	if args.Type != "" {
		query["type"] = args.Type
	}

	resp := make([]*models.Favorite, 0)
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *FavoriteColl) Find(userID int, name, Type string) (*models.Favorite, error) {
	resp := new(models.Favorite)
	query := bson.M{"user_id": userID, "name": name}
	if Type != "" {
		query["type"] = Type
	}

	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *FavoriteColl) Delete(args *FavoriteArgs) error {
	query := bson.M{"user_id": args.UserID, "product_name": args.ProductName, "name": args.Name, "type": args.Type}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
