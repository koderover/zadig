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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LarkPluginWorkflowConfig struct {
	ID              primitive.ObjectID              `bson:"_id,omitempty"                json:"id,omitempty"`
	WorkspaceID     string                          `bson:"workspace_id" json:"workspace_id"`
	WorkItemTypeKey string                          `bson:"work_item_type_key" json:"work_item_type_key"`
	Nodes           []*LarkPluginWorkflowConfigNode `bson:"nodes" json:"nodes"`
	UpdateTime      int64                           `bson:"update_time" json:"update_time"`
}

type LarkPluginWorkflowConfigNode struct {
	TemplateID   int64  `bson:"template_id" json:"template_id"`
	TemplateName string `bson:"template_name" json:"template_name"`
	NodeID       string `bson:"node_id" json:"node_id"`
	NodeName     string `bson:"node_name" json:"node_name"`
	ProjectKey   string `bson:"project_key" json:"project_key"`
	WorkflowName string `bson:"workflow_name" json:"workflow_name"`
}

func (LarkPluginWorkflowConfig) TableName() string {
	return "lark_plugin_workflow_config"
}
