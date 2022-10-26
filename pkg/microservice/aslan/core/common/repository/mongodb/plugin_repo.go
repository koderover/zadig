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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type PluginRepoColl struct {
	*mongo.Collection

	coll string
}

func NewPluginRepoColl() *PluginRepoColl {
	name := models.PluginRepo{}.TableName()
	return &PluginRepoColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PluginRepoColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *PluginRepoColl) GetCollectionName() string {
	return c.coll
}

func (c *PluginRepoColl) List(offical *bool) ([]*models.PluginRepo, error) {
	var res []*models.PluginRepo
	query := bson.M{}
	if offical != nil {
		query["is_offical"] = *offical
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)

	return res, err
}

func (c *PluginRepoColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *PluginRepoColl) Upsert(pluginRepo *models.PluginRepo) error {
	query := bson.M{"repo_name": pluginRepo.RepoName, "repo_owner": pluginRepo.RepoOwner, "branch": pluginRepo.Branch}
	change := bson.M{"$set": pluginRepo}
	pluginRepo.UpdateTime = time.Now().Unix()

	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}
