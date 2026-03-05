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

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type LarkPluginReleaseWorkItemBind struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	WorkspaceID     string             `bson:"workspace_id"           json:"workspace_id"`

	ReleaseItemID      string          `bson:"release_item_id"        json:"release_item_id"`
	// redundant field, for data integrity. should be the same as the configured release stage item type key.
	ReleaseItemTypeKey string          `bson:"release_item_type_key"  json:"release_item_type_key"`

	WorkItemTypeKey string             `bson:"work_item_type_key"     json:"work_item_type_key"`
	WorkItemID      string             `bson:"work_item_id"           json:"work_item_id"`
	WorkItemName    string             `bson:"work_item_name"         json:"work_item_name"`
}

func (LarkPluginReleaseWorkItemBind) TableName() string {
	return "lark_plugin_release_workitem_bind"
}