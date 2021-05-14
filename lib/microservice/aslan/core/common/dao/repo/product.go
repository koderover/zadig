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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ProductFindOptions struct {
	Name    string
	EnvName string
}

// ClusterId is a primitive.ObjectID{}.Hex()
type ProductListOptions struct {
	EnvName       string
	Name          string
	IsPublic      bool
	ClusterId     string
	IsSort        bool
	ExcludeStatus string
	ExcludeSource string
	Source        string
}

type ProductColl struct {
	*mongo.Collection

	coll string
}

func NewProductColl() *ProductColl {
	name := models.Product{}.TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProductColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "update_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

type ProductEnvFindOptions struct {
	Name      string
	Namespace string
}

func (c *ProductColl) FindEnv(opt *ProductEnvFindOptions) (*models.Product, error) {
	query := bson.M{}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}

	if opt.Namespace != "" {
		query["namespace"] = opt.Namespace
	}

	ret := new(models.Product)
	err := c.FindOne(context.TODO(), query).Decode(ret)
	return ret, err
}

func (c *ProductColl) Find(opt *ProductFindOptions) (*models.Product, error) {
	res := &models.Product{}
	query := bson.M{}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	}

	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *ProductColl) List(opt *ProductListOptions) ([]*models.Product, error) {
	var ret []*models.Product
	query := bson.M{}

	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}
	if opt.IsPublic {
		query["is_public"] = opt.IsPublic
	}
	if opt.ClusterId != "" {
		query["cluster_id"] = opt.ClusterId
	}
	if opt.Source != "" {
		query["source"] = opt.Source
	}
	if opt.ExcludeSource != "" {
		query["source"] = bson.M{"$ne": opt.ExcludeSource}
	}
	if opt.ExcludeStatus != "" {
		query["status"] = bson.M{"$ne": opt.ExcludeStatus}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *ProductColl) UpdateStatus(owner, productName, status string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"status": status,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) UpdateErrors(owner, productName, errorMsg string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"error": errorMsg,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) Delete(owner, productName string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

// Update  Cannot update owner & product name
func (c *ProductColl) Update(args *models.Product) error {
	query := bson.M{"env_name": args.EnvName, "product_name": args.ProductName}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"services":    args.Services,
		"status":      args.Status,
		"revision":    args.Revision,
		"render":      args.Render,
		"error":       args.Error,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) Create(args *models.Product) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil Product")
	}

	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now
	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *ProductColl) UpdateGroup(envName, productName string, groupIndex int, group []*models.ProductService) error {
	serviceGroup := fmt.Sprintf("services.%d", groupIndex)
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
	}
	change := bson.M{
		"update_time": time.Now().Unix(),
		serviceGroup:  group,
	}

	_, err := c.UpdateOne(context.TODO(), query, bson.M{"$set": change})

	return err
}

func (c *ProductColl) UpdateIsPublic(envName, productName string, isPublic bool) error {
	query := bson.M{"env_name": envName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"is_public":   isPublic,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}
