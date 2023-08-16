/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type ReleasePlan struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Index     int                `bson:"index"       yaml:"index"                   json:"index"`
	Name      string             `bson:"name"       yaml:"name"                   json:"name"`
	Principal string             `bson:"principal"       yaml:"principal"                   json:"principal"`
	// PrincipalID is the user id of the principal
	PrincipalID string `bson:"principal_id"       yaml:"principal_id"                   json:"principal_id"`
	StartTime   int64  `bson:"start_time"       yaml:"start_time"                   json:"start_time"`
	EndTime     int64  `bson:"end_time"       yaml:"end_time"                   json:"end_time"`
	Description string `bson:"description"       yaml:"description"                   json:"description"`
	CreatedBy   string `bson:"created_by"       yaml:"created_by"                   json:"created_by"`
	CreateTime  int64  `bson:"create_time"       yaml:"create_time"                   json:"create_time"`
	UpdatedBy   string `bson:"updated_by"       yaml:"updated_by"                   json:"updated_by"`
	UpdateTime  int64  `bson:"update_time"       yaml:"update_time"                   json:"update_time"`

	Status config.ReleasePlanStatus `bson:"status"       yaml:"status"                   json:"status"`
}

type ReleaseJob struct {
	Name   string        `bson:"name"       yaml:"name"                   json:"name"`
	Type   string        `bson:"type"       yaml:"type"                   json:"type"`
	Status config.Status `bson:"status"       yaml:"status"                   json:"status"`
	Spec   interface{}   `bson:"spec"       yaml:"spec"                   json:"spec"`
}

type TextReleaseJobSpec struct {
	Content string `bson:"content"       yaml:"content"                   json:"content"`
}

type WorkflowReleaseJobSpec struct {
	ProjectName  string `bson:"project_name"       yaml:"project_name"                   json:"project_name"`
	WorkflowName string `bson:"workflow_name"       yaml:"workflow_name"                   json:"workflow_name"`
}
