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

package models

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Sprint struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"      yaml:"-"                  json:"id"`
	Name        string              `bson:"name"               yaml:"name"               json:"name"`
	Key         string              `bson:"key"                yaml:"key"                json:"key"`
	KeyInitials string              `bson:"key_initials"       yaml:"key_initials"       json:"key_initials"`
	TemplateID  string              `bson:"template_id"        yaml:"template_id"        json:"template_id"`
	ProjectName string              `bson:"project_name"       yaml:"project_name"       json:"project_name"`
	Stages      []*SprintStage      `bson:"stages"             yaml:"stages"             json:"stages"`
	IsArchived  bool                `bson:"is_archived"        yaml:"is_archived"        json:"is_archived"`
	CreatedBy   types.UserBriefInfo `bson:"created_by"         yaml:"created_by"         json:"created_by"`
	CreateTime  int64               `bson:"create_time"        yaml:"create_time"        json:"create_time"`
	UpdatedBy   types.UserBriefInfo `bson:"updated_by"         yaml:"updated_by"         json:"updated_by"`
	UpdateTime  int64               `bson:"update_time"        yaml:"update_time"        json:"update_time"`
}

type SprintStage struct {
	ID          string            `bson:"id"             yaml:"id"                         json:"id"`
	Name        string            `bson:"name"           yaml:"name"                       json:"name"`
	Workflows   []*SprintWorkflow `bson:"workflows"      yaml:"workflows"                  json:"workflows"`
	WorkItemIDs []string          `bson:"workitem_ids"   yaml:"workitem_ids"               json:"workitem_ids"`
}

func (Sprint) TableName() string {
	return "sprint"
}

func (s *Sprint) Lint() error {
	stageNameSet := sets.NewString()
	for _, stage := range s.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage %s's ID is required", stage.Name)
		}
		if stageNameSet.Has(stage.Name) {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageNameSet.Insert(stage.Name)
	}
	return nil
}
