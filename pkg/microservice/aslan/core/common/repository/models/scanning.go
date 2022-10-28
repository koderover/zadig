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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/types"
)

type Scanning struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string              `bson:"name"          json:"name"`
	ProjectName string              `bson:"project_name"  json:"project_name"`
	Description string              `bson:"description"   json:"description"`
	ScannerType string              `bson:"scanner_type"  json:"scanner_type"`
	ImageID     string              `bson:"image_id"      json:"image_id"`
	SonarID     string              `bson:"sonar_id"      json:"sonar_id"`
	Repos       []*types.Repository `bson:"repos"         json:"repos"`
	Installs    []*Item             `bson:"installs"      json:"installs"`
	PreScript   string              `bson:"pre_script"    json:"pre_script"`
	// Parameter is for sonarQube type only
	Parameter string `bson:"parameter" json:"parameter"`
	// Script is for other type only
	Script           string                         `bson:"script"                json:"script"`
	AdvancedSetting  *types.ScanningAdvancedSetting `bson:"advanced_setting"      json:"advanced_setting"`
	CheckQualityGate bool                           `bson:"check_quality_gate"    json:"check_quality_gate"`

	CreatedAt int64  `bson:"created_at" json:"created_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
	UpdatedBy string `bson:"updated_by" json:"updated_by"`
}

func (Scanning) TableName() string {
	return "scanning"
}
