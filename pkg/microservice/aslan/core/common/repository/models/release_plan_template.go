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
)

type ReleasePlanTemplate struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	TemplateName string             `bson:"template_name"       yaml:"template_name"      json:"template_name"`
	CreatedBy    string             `bson:"created_by"       yaml:"created_by"                   json:"created_by"`
	CreateTime   int64              `bson:"create_time"       yaml:"create_time"                   json:"create_time"`
	UpdatedBy    string             `bson:"updated_by"       yaml:"updated_by"                   json:"updated_by"`
	UpdateTime   int64              `bson:"update_time"       yaml:"update_time"                   json:"update_time"`

	Approval *Approval `bson:"approval"       yaml:"approval"                   json:"approval,omitempty"`

	Jobs []*ReleaseJob `bson:"jobs"       yaml:"jobs"                   json:"jobs"`
}

func (ReleasePlanTemplate) TableName() string {
	return "release_plan_template"
}
