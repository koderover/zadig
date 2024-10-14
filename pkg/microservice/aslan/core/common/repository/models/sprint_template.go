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

	"github.com/google/uuid"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"k8s.io/apimachinery/pkg/util/sets"
)

type SprintTemplate struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty"    yaml:"-"                  json:"id"`
	Name        string                 `bson:"name"             yaml:"name"               json:"name"`
	Key         string                 `bson:"key"              yaml:"key"                json:"key"`
	KeyInitials string                 `bson:"key_initials"     yaml:"key_initials"      json:"key_initials"`
	ProjectName string                 `bson:"project_name"     yaml:"project_name"       json:"project_name"`
	CreatedBy   types.UserBriefInfo    `bson:"created_by"       yaml:"created_by"         json:"created_by"`
	CreateTime  int64                  `bson:"create_time"      yaml:"create_time"        json:"create_time"`
	UpdatedBy   types.UserBriefInfo    `bson:"updated_by"       yaml:"updated_by"         json:"updated_by"`
	UpdateTime  int64                  `bson:"update_time"      yaml:"update_time"        json:"update_time"`
	Stages      []*SprintStageTemplate `bson:"stages"           yaml:"stages"             json:"stages"`
}

type SprintStageTemplate struct {
	ID        string            `bson:"id"             yaml:"id"                         json:"id"`
	Name      string            `bson:"name"           yaml:"name"                       json:"name"`
	Workflows []*SprintWorkflow `bson:"workflows"      yaml:"workflows"                  json:"workflows"`
}

type SprintWorkflow struct {
	Name        string `bson:"name"         yaml:"name"         json:"name"`
	DisplayName string `bson:"display_name" yaml:"display_name" json:"display_name"`
	IsDeleted   bool   `bson:"is_deleted"   yaml:"is_deleted"   json:"is_deleted"`
}

func (SprintTemplate) TableName() string {
	return "sprint_template"
}

func (s *SprintTemplate) Lint() error {
	s.Key, s.KeyInitials = util.GetKeyAndInitials(s.Name)
	stageNameSet := sets.NewString()
	stageIDSet := sets.NewString()
	for _, stage := range s.Stages {
		if stage.ID == "" {
			stage.ID = uuid.NewString()
		}

		if stageIDSet.Has(stage.ID) {
			return fmt.Errorf("阶段重复 id: %s, name: %s", stage.ID, stage.Name)
		}
		if stageNameSet.Has(stage.Name) {
			return fmt.Errorf("阶段名称重复: %s", stage.Name)
		}
		stageNameSet.Insert(stage.Name)
		stageIDSet.Insert(stage.ID)
	}
	return nil
}
