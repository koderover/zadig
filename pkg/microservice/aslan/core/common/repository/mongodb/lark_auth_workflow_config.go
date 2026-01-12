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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LarkPluginAuthConfigColl struct {
	*mongo.Collection

	coll string
}

func NewLarkPluginAuthConfigColl() *LarkPluginAuthConfigColl {
	name := models.LarkPluginAuthConfig{}.TableName()
	return &LarkPluginAuthConfigColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LarkPluginAuthConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginAuthConfigColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *LarkPluginAuthConfigColl) Get(workspaceID string) (*models.LarkPluginAuthConfig, error) {
	resp := new(models.LarkPluginAuthConfig)
	cursor, err := c.Collection.Find(context.TODO(), bson.M{"workspace_id": workspaceID})
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *LarkPluginAuthConfigColl) Update(config *models.LarkPluginAuthConfig) error {
	if config == nil {
		return nil
	}

	query := bson.M{"workspace_id": config.WorkspaceID}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, config, opts)

	return err
}
