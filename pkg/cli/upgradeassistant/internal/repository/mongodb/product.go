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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ProductFindOptions struct {
	Name      string
	EnvName   string
	Namespace string
}

// ClusterId is a primitive.ObjectID{}.Hex()
type ProductListOptions struct {
	EnvName             string
	Name                string
	IsPublic            bool
	ClusterID           string
	IsSortByUpdateTime  bool
	IsSortByProductName bool
	ExcludeStatus       string
	ExcludeSource       string
	Source              string
	InProjects          []string
	InEnvs              []string
}

type ProductColl struct {
	*mongo.Collection

	coll string
}

func NewProductColl() *ProductColl {
	name := (&models.Product{}).TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProductColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductColl) List(opt *ProductListOptions) ([]*models.Product, error) {
	var ret []*models.Product
	query := bson.M{}

	if opt == nil {
		opt = &ProductListOptions{}
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	} else if len(opt.InEnvs) > 0 {
		query["env_name"] = bson.M{"$in": opt.InEnvs}
	}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}
	if opt.IsPublic {
		query["is_public"] = opt.IsPublic
	}
	if opt.ClusterID != "" {
		query["cluster_id"] = opt.ClusterID
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
	if len(opt.InProjects) > 0 {
		query["product_name"] = bson.M{"$in": opt.InProjects}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsSortByUpdateTime {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	if opt.IsSortByProductName {
		opts.SetSort(bson.D{{"product_name", 1}})
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *ProductColl) UpdateAllRegistry(envs []*models.Product) error {
	if len(envs) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, env := range envs {
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", env.ID}}).
				SetUpdate(bson.D{{"$set", bson.D{{"registry_id", env.RegistryID}}}}),
		)
	}
	_, err := c.BulkWrite(context.TODO(), ms)

	return err
}

func (c *ProductColl) UpdateProductRender(product *models.Product) error {
	query := bson.M{"_id": product.ID}
	change := bson.M{"$set": bson.M{
		"render": product.Render,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	return err
}
