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

type LarkPluginStageConfigV2 struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"      json:"id,omitempty"`
	StageName         string             `bson:"stage_name"         json:"stage_name"`
	WorkspaceID       string             `bson:"workspace_id"       json:"workspace_id"`
	UpdateTime        int64              `bson:"update_time"        json:"update_time"`
	CodeSource        string             `bson:"code_source"        json:"code_source"`
	BranchFilter      string             `bson:"branch_filter"      json:"branch_filter"`
	TargetBranch      string             `bson:"target_branch"      json:"target_branch"`

	// work item should only be bound to stage when the stageName is release (yes currently it is hard coded, remember to deal with it)
	WorkItemTypeKey   string             `bson:"work_item_type_key" json:"work_item_type_key"`
	WorkItemType      string             `bson:"work_item_type"     json:"work_item_type"`
}

func (LarkPluginStageConfigV2) TableName() string {
	return "lark_plugin_stage_config_v2"
}

type LarkPluginWorkflowConfigV2 struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	// stageName/workspaceID is the foreign key combination to link to the LarkPluginWorkflowConfigV2
	StageName    string             `bson:"stage_name"       json:"stage_name"`
	WorkspaceID  string             `bson:"workspace_id"     json:"workspace_id"`
	// when the stage name is not release, work item type/templateID/nodeID together forms the unique node setting
	WorkItemTypeKey   string             `bson:"work_item_type_key" json:"work_item_type_key"`
	WorkItemType      string             `bson:"work_item_type"     json:"work_item_type"`
	TemplateID        int64              `bson:"template_id"        json:"template_id"`
	TemplateName      string             `bson:"template_name"      json:"template_name"`
	NodeID            string             `bson:"node_id"            json:"node_id"`
	NodeName          string             `bson:"node_name"          json:"node_name"`
	
	ProjectKey   string             `bson:"project_key"      json:"project_key"`
	WorkflowName string             `bson:"workflow_name"    json:"workflow_name"`
}

func (LarkPluginWorkflowConfigV2) TableName() string {
	return "lark_plugin_workflow_config_v2"
}
