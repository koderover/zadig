/*
Copyright 2022 The KodeRover Authors.

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

type EnvResourceColl struct {
	*mongo.Collection
	coll string
}

func NewEnvResourceColl() *EnvResourceColl {
	name := models.EnvResource{}.TableName()
	return &EnvResourceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvResourceColl) BatchInsert(resources []*models.EnvResource) error {
	if len(resources) == 0 {
		return nil
	}
	resList := make([]interface{}, 0, len(resources))
	for _, res := range resources {
		resList = append(resList, res)
	}
	_, err := c.InsertMany(context.TODO(), resList)
	return err
}

type ConfigMapColl struct {
	*mongo.Collection
	coll string
}

func NewConfigMapColl() *ConfigMapColl {
	name := models.EnvConfigMap{}.TableName()
	return &ConfigMapColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ConfigMapColl) GetCollectionName() string {
	return c.coll
}

func (c *ConfigMapColl) List() ([]*models.EnvConfigMap, error) {
	query := bson.M{}

	var resp []*models.EnvConfigMap
	ctx := context.Background()
	opts := options.Find()

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	return resp, err
}

type IngressColl struct {
	*mongo.Collection

	coll string
}

func NewIngressColl() *IngressColl {
	name := models.EnvIngress{}.TableName()
	return &IngressColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *IngressColl) GetCollectionName() string {
	return c.coll
}

func (c *IngressColl) List() ([]*models.EnvIngress, error) {
	query := bson.M{}

	var resp []*models.EnvIngress
	ctx := context.Background()
	opts := options.Find()
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	return resp, err
}

type PvcColl struct {
	*mongo.Collection

	coll string
}

func NewPvcColl() *PvcColl {
	name := models.EnvPvc{}.TableName()
	return &PvcColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *PvcColl) GetCollectionName() string {
	return c.coll
}

func (c *PvcColl) List() ([]*models.EnvPvc, error) {
	query := bson.M{}

	var resp []*models.EnvPvc
	ctx := context.Background()
	opts := options.Find()
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	return resp, err
}

type SecretColl struct {
	*mongo.Collection

	coll string
}

func NewSecretColl() *SecretColl {
	name := models.EnvSecret{}.TableName()
	return &SecretColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *SecretColl) GetCollectionName() string {
	return c.coll
}

func (c *SecretColl) List() ([]*models.EnvSecret, error) {
	query := bson.M{}

	var resp []*models.EnvSecret
	ctx := context.Background()
	opts := options.Find()
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	return resp, err
}
