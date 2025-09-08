/*
Copyright 2025 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type PluginColl struct {
	*mongo.Collection
	coll string
}

func NewPluginColl() *PluginColl {
	name := models.Plugin{}.TableName()
	return &PluginColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *PluginColl) GetCollectionName() string { return c.coll }

func (c *PluginColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{bson.E{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)}
	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *PluginColl) List() ([]*models.Plugin, error) {
	resp := make([]*models.Plugin, 0)
	cursor, err := c.Collection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	if err = cursor.All(context.Background(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *PluginColl) Get(id string) (*models.Plugin, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	res := new(models.Plugin)
	if err := c.FindOne(context.Background(), bson.M{"_id": oid}).Decode(res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *PluginColl) Create(m *models.Plugin) error {
	if m == nil {
		return errors.New("nil plugin")
	}
	m.CreateTime = time.Now().Unix()
	m.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(context.Background(), m)
	return err
}

func (c *PluginColl) Update(id string, m *models.Plugin) error {
	if m == nil {
		return errors.New("nil plugin")
	}
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	change := bson.M{"$set": bson.M{
		"name":        m.Name,
		"index":       m.Index,
		"type":        m.Type,
		"description": m.Description,
		"filters":     m.Filters,
		"storage_id":  m.StorageID,
		"file_path":   m.FilePath,
		"file_name":   m.FileName,
		"file_size":   m.FileSize,
		"file_hash":   m.FileHash,
		"enabled":     m.Enabled,
		"route":       m.Route,
		"update_by":   m.UpdateBy,
		"update_time": time.Now().Unix(),
	}}
	_, err = c.UpdateOne(context.Background(), bson.M{"_id": oid}, change)
	return err
}

func (c *PluginColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = c.DeleteOne(context.Background(), bson.M{"_id": oid})
	return err
}
