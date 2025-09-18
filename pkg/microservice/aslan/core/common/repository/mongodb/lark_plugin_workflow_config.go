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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LarkPluginWorkflowConfigColl struct {
	*mongo.Collection

	coll string
}

func NewLarkPluginWorkflowConfigColl() *LarkPluginWorkflowConfigColl {
	name := models.LarkPluginWorkflowConfig{}.TableName()
	return &LarkPluginWorkflowConfigColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LarkPluginWorkflowConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginWorkflowConfigColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "work_item_type_key", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *LarkPluginWorkflowConfigColl) Get(workspaceID string) ([]*models.LarkPluginWorkflowConfig, error) {
	resp := make([]*models.LarkPluginWorkflowConfig, 0)
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

func (c *LarkPluginWorkflowConfigColl) GetWorkItemTypeConfig(workspaceID, workItemType string) (*models.LarkPluginWorkflowConfig, error) {
	resp := new(models.LarkPluginWorkflowConfig)

	query := bson.M{"workspace_id": workspaceID, "work_item_type_key": workItemType}
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *LarkPluginWorkflowConfigColl) Update(configs []*models.LarkPluginWorkflowConfig) error {
	if len(configs) == 0 {
		return nil
	}

	// 构建批量操作
	operations := make([]mongo.WriteModel, 0, len(configs))
	for _, config := range configs {
		config.UpdateTime = time.Now().Unix()
		// 使用 WorkspaceID 和 WorkItemType 作为唯一标识字段进行 upsert 操作
		query := bson.M{"workspace_id": config.WorkspaceID, "work_item_type_key": config.WorkItemTypeKey}
		operation := mongo.NewReplaceOneModel().
			SetFilter(query).
			SetReplacement(config).
			SetUpsert(true)
		operations = append(operations, operation)
	}

	// 执行批量操作
	opts := options.BulkWrite().SetOrdered(false)
	_, err := c.BulkWrite(context.TODO(), operations, opts)

	return err
}

func (c *LarkPluginWorkflowConfigColl) Delete(workItemTypeKeys []string) error {
	if len(workItemTypeKeys) == 0 {
		return nil
	}

	query := bson.M{"work_item_type_key": bson.M{"$in": workItemTypeKeys}}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}
