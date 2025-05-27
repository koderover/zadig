/*
Copyright 2023 The KodeRover Authors.

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

type Migration struct {
	ID                               primitive.ObjectID `bson:"_id,omitempty"`
	SonarMigration                   bool               `bson:"sonar_migration"`
	UpdateWorkflow340JobSpec         bool               `bson:"update_workflow_340_job_spec"`
	UpdateWorkflow340JobTemplateSpec bool               `bson:"update_workflow_340_job_template_spec"`
	WorkflowV4350HookMigration       bool               `bson:"workflow_v4_350_hook_migration"`
	Migration350VMDeploy             bool               `bson:"migration_350_vm_deploy"`
	UpdateLarkEventSetting           bool               `bson:"update_lark_event_setting"`
}

func (Migration) TableName() string {
	return "migration"
}
