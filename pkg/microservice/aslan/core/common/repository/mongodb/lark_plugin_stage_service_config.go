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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LarkPluginWorkItemStageWorkflowInputConfigColl struct {
	*mongo.Collection
	coll string
}

func NewLarkPluginWorkItemStageWorkflowInputConfigColl() *LarkPluginWorkItemStageWorkflowInputConfigColl {
	name := models.LarkPluginWorkItemStageWorkflowInputConfig{}.TableName()
	return &LarkPluginWorkItemStageWorkflowInputConfigColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LarkPluginWorkItemStageWorkflowInputConfigColl) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginWorkItemStageWorkflowInputConfigColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "stage_name", Value: 1},
				bson.E{Key: "work_item_type_key", Value: 1},
				bson.E{Key: "work_item_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "stage_name", Value: 1},
				bson.E{Key: "work_item_type_key", Value: 1},
				bson.E{Key: "work_item_id", Value: 1},
				bson.E{Key: "template_id", Value: 1},
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "service_module", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *LarkPluginWorkItemStageWorkflowInputConfigColl) GetByWorkItem(workspaceID, stageName, workItemTypeKey, workItemID string) ([]*models.LarkPluginWorkItemStageWorkflowInputConfig, error) {
	resp := make([]*models.LarkPluginWorkItemStageWorkflowInputConfig, 0)
	query := bson.M{
		"workspace_id":       workspaceID,
		"stage_name":         stageName,
		"work_item_type_key": workItemTypeKey,
		"work_item_id":       workItemID,
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(context.TODO(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *LarkPluginWorkItemStageWorkflowInputConfigColl) ReplaceByWorkItem(workspaceID, stageName, workItemTypeKey string, workItemID string, configs []*models.LarkPluginWorkItemStageWorkflowInputConfig) error {
	query := bson.M{
		"workspace_id":       workspaceID,
		"stage_name":         stageName,
		"work_item_type_key": workItemTypeKey,
		"work_item_id":       workItemID,
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)
	if err != nil {
		return err
	}

	if len(configs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	docs := make([]interface{}, 0, len(configs))
	for _, cfg := range configs {
		cfg.UpdateTime = now
		docs = append(docs, cfg)
	}
	_, err = c.Collection.InsertMany(context.TODO(), docs)
	return err
}
