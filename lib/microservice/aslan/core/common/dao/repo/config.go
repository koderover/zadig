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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ConfigFindOption struct {
	ConfigName  string
	ServiceName string
	ProductName string
	Revision    int64
}

type ConfigColl struct {
	*mongo.Collection

	coll string
}

func NewConfigColl() *ConfigColl {
	name := models.Config{}.TableName()
	return &ConfigColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *ConfigColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "config_name", Value: 1},
			bson.E{Key: "service_name", Value: 1},
			bson.E{Key: "revision", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

// Find 查询特定版本的配置模板
// 如果 Revision == 0 查询最大版本的配置模板
func (c *ConfigColl) Find(opt *ConfigFindOption) (*models.Config, error) {
	if opt == nil {
		return nil, errors.New("nil ConfigFindOption")
	}

	resp := &models.Config{}

	query := bson.M{"config_name": opt.ConfigName, "service_name": opt.ServiceName}

	opts := options.FindOne()
	if opt.Revision > 0 {
		query["revision"] = opt.Revision
	} else {
		opts.SetSort(bson.D{{"revision", -1}})
	}

	err := c.FindOne(context.TODO(), query, opts).Decode(resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// ListMaxRevisions 查询最新版本的配置模板
func (c *ConfigColl) ListMaxRevisions(productName string) ([]*models.Config, error) {
	var pipeResp []*models.ConfigPipeResp
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id": bson.M{
					"config_name":  "$config_name",
					"service_name": "$service_name",
					"product_name": "$product_name",
				},
				"revision": bson.M{"$max": "$revision"},
			},
		},
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err := cursor.All(context.TODO(), &pipeResp); err != nil {
		return nil, err
	}

	var resp []*models.Config
	for _, pipe := range pipeResp {
		opt := &ConfigFindOption{
			ConfigName:  pipe.Config.ConfigName,
			ServiceName: pipe.Config.ServiceName,
			ProductName: pipe.Config.ProductName,
			Revision:    pipe.Revision,
		}
		rs, err := c.Find(opt)
		if err != nil {
			return nil, err
		}
		resp = append(resp, rs)
	}

	return resp, nil
}

// ListAllRevisions 列出历史所有的service
func (c *ConfigColl) ListAllRevisions() ([]*models.Config, error) {
	resp := make([]*models.Config, 0)
	query := bson.M{}
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
