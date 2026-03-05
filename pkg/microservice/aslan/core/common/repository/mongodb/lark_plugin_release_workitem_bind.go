/*
Copyright 2026 The KodeRover Authors.

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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LarkPluginReleaseWorkItemBindColl struct {
	*mongo.Collection
	coll string
}

func NewLarkPluginReleaseWorkItemBindColl() *LarkPluginReleaseWorkItemBindColl {
	name := models.LarkPluginReleaseWorkItemBind{}.TableName()
	return &LarkPluginReleaseWorkItemBindColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LarkPluginReleaseWorkItemBindColl) GetCollectionName() string {
	return c.coll
}

func (c *LarkPluginReleaseWorkItemBindColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			// one workitem can only be bound once
			Keys: bson.D{
				bson.E{Key: "workspace_id", Value: 1},
				bson.E{Key: "work_item_type_key", Value: 1},
				bson.E{Key: "work_item_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *LarkPluginReleaseWorkItemBindColl) CreateWorkItemBind(bind *models.LarkPluginReleaseWorkItemBind) error {
	if bind == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), bind)
	return err
}

func (c *LarkPluginReleaseWorkItemBindColl) GetWorkItemBind(workspaceID, workItemTypeKey, workItemID string) (*models.LarkPluginReleaseWorkItemBind, error) {
	if workspaceID == "" || workItemTypeKey == "" || workItemID == "" {
		return nil, fmt.Errorf("workspaceID, workItemTypeKey and workItemID are required")
	}

	var bind models.LarkPluginReleaseWorkItemBind
	err := c.FindOne(context.TODO(), bson.M{"workspace_id": workspaceID, "work_item_type_key": workItemTypeKey, "work_item_id": workItemID}).Decode(&bind)
	if err != nil {
		return nil, err
	}

	return &bind, nil
}

func (c *LarkPluginReleaseWorkItemBindColl) ListReleaseBindItems(workspaceID, releaseItemID string) ([]*models.LarkPluginReleaseWorkItemBind, error) {
	if workspaceID == "" || releaseItemID == "" {
		return nil, fmt.Errorf("workspaceID and releaseItemID are required")
	}

	var bindItemList []*models.LarkPluginReleaseWorkItemBind
	cursor, err := c.Find(context.TODO(), bson.M{
		"workspace_id": workspaceID, 
		"release_item_id": releaseItemID, 
	})
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &bindItemList)
	if err != nil {
		return nil, err
	}

	return bindItemList, nil
}


