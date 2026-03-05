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

// ---------------------------------------------------------------------------
// LarkPluginWorkflowConfigV2Coll – lark_plugin_workflow_config_v2
// ---------------------------------------------------------------------------

type LarkPluginStageConfigV2Coll struct {
	*mongo.Collection

	coll string
}

func NewLarkPluginStageConfigV2Coll() *LarkPluginStageConfigV2Coll {
	name := models.LarkPluginStageConfigV2{}.TableName()
	return &LarkPluginStageConfigV2Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LarkPluginStageConfigV2Coll) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginStageConfigV2Coll) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			// each stage is unique under the same workspace
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "stage_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *LarkPluginStageConfigV2Coll) GetByStage(workspaceID, stageName string) (*models.LarkPluginStageConfigV2, error) {
	resp := new(models.LarkPluginStageConfigV2)
	query := bson.M{"workspace_id": workspaceID, "stage_name": stageName}
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *LarkPluginStageConfigV2Coll) Upsert(cfg *models.LarkPluginStageConfigV2) error {
	cfg.UpdateTime = time.Now().Unix()
	query := bson.M{"workspace_id": cfg.WorkspaceID, "stage_name": cfg.StageName}
	opts := options.Replace().SetUpsert(true)
	_, err := c.Collection.ReplaceOne(context.TODO(), query, cfg, opts)
	return err
}

func (c *LarkPluginStageConfigV2Coll) Delete(workspaceID, stageName string) error {
	query := bson.M{"workspace_id": workspaceID, "stage_name": stageName}
	_, err := c.Collection.DeleteOne(context.TODO(), query)
	return err
}

// ---------------------------------------------------------------------------
// LarkPluginWfConfigNodeV2Coll – lark_plugin_wf_config_node_v2
// ---------------------------------------------------------------------------

type LarkPluginWorkflowConfigV2Coll struct {
	*mongo.Collection
	coll string
}

func NewLarkPluginWorkflowConfigV2Coll() *LarkPluginWorkflowConfigV2Coll {
	name := models.LarkPluginWorkflowConfigV2{}.TableName()
	return &LarkPluginWorkflowConfigV2Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LarkPluginWorkflowConfigV2Coll) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginWorkflowConfigV2Coll) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "stage_name", Value: 1},
			},
		},
		{
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "stage_name", Value: 1},
				bson.E{Key: "template_id", Value: 1},
				bson.E{Key: "node_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}



func (c *LarkPluginWorkflowConfigV2Coll) Find(workspaceID, workItemType string, templateID int64, nodeID string) (*models.LarkPluginWorkflowConfigV2, error) {
	resp := new(models.LarkPluginWorkflowConfigV2)
	query := bson.M{
		"workspace_id": workspaceID,
		"work_item_type": workItemType,
		"template_id": templateID,
		"node_id": nodeID,
	}

	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type ListWorkflowConfigV2Args struct {
	WorkspaceID string
	WorkItemTypeKey string
	TemplateID int64
	NodeID string
	StageName string
}

func (c *LarkPluginWorkflowConfigV2Coll) List(args *ListWorkflowConfigV2Args) ([]*models.LarkPluginWorkflowConfigV2, error) {
	resp := make([]*models.LarkPluginWorkflowConfigV2, 0)
	query := bson.M{}
	if args.WorkspaceID != "" {
		query["workspace_id"] = args.WorkspaceID
	}
	if args.WorkItemTypeKey != "" {
		query["work_item_type_key"] = args.WorkItemTypeKey
	}
	if args.TemplateID != 0 {
		query["template_id"] = args.TemplateID
	}
	if args.NodeID != "" {
		query["node_id"] = args.NodeID
	}
	if args.StageName != "" {
		query["stage_name"] = args.StageName
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

func (c *LarkPluginWorkflowConfigV2Coll) GetByStage(workspaceID, stageName string) ([]*models.LarkPluginWorkflowConfigV2, error) {
	resp := make([]*models.LarkPluginWorkflowConfigV2, 0)
	query := bson.M{"workspace_id": workspaceID, "stage_name": stageName}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(context.TODO(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *LarkPluginWorkflowConfigV2Coll) ReplaceByStage(workspaceID, stageName string, nodes []*models.LarkPluginWorkflowConfigV2) error {
	query := bson.M{"workspace_id": workspaceID, "stage_name": stageName}
	_, err := c.Collection.DeleteMany(context.TODO(), query)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return nil
	}

	docs := make([]interface{}, 0, len(nodes))
	for _, node := range nodes {
		node.StageName = stageName
		node.WorkspaceID = workspaceID
		docs = append(docs, node)
	}
	_, err = c.Collection.InsertMany(context.TODO(), docs)
	return err
}

func (c *LarkPluginWorkflowConfigV2Coll) DeleteByStage(workspaceID, stageName string) error {
	query := bson.M{"workspace_id": workspaceID, "stage_name": stageName}
	_, err := c.Collection.DeleteMany(context.TODO(), query)
	return err
}

func (c *LarkPluginWorkflowConfigV2Coll) GetByWorkItem(workspaceID string, templateID int64, nodeIDs []string) ([]*models.LarkPluginWorkflowConfigV2, error) {
	resp := make([]*models.LarkPluginWorkflowConfigV2, 0)
	query := bson.M{
		"workspace_id": workspaceID,
		"template_id":  templateID,
		"node_id":      bson.M{"$in": nodeIDs},
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
