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
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScanningTemplate struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string             `bson:"name"          json:"name"`
	ScannerType string             `bson:"scanner_type"  json:"scanner_type"`
	// EnableScanner indicates whether user uses sonar scanner instead of the script
	EnableScanner  bool     `bson:"enable_scanner" json:"enable_scanner"`
	ImageID        string   `bson:"image_id"      json:"image_id"`
	SonarID        string   `bson:"sonar_id"      json:"sonar_id"`
	Installs       []*Item  `bson:"installs"      json:"installs"`
	Infrastructure string   `bson:"infrastructure"           json:"infrastructure"`
	VMLabels       []string `bson:"vm_labels"                json:"vm_labels"`
	// Parameter is for sonarQube type only
	Parameter string `bson:"parameter" json:"parameter"`
	// Envs is the user defined key/values
	Envs KeyValList `bson:"envs" json:"envs"`
	// Script is for other type only
	ScriptType       types.ScriptType         `bson:"script_type"           json:"script_type"`
	Script           string                   `bson:"script"                json:"script"`
	AdvancedSetting  *ScanningAdvancedSetting `bson:"advanced_settings"      json:"advanced_settings"`
	CheckQualityGate bool                     `bson:"check_quality_gate"    json:"check_quality_gate"`

	CreatedAt int64  `bson:"created_at" json:"created_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
	UpdatedBy string `bson:"updated_by" json:"updated_by"`
}

func (ScanningTemplate) TableName() string {
	return "scanning_template"
}
