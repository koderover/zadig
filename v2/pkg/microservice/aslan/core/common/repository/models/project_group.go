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

import "go.mongodb.org/mongo-driver/bson/primitive"

type ProjectGroup struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Name        string             `bson:"name"                   json:"name"`
	Projects    []*ProjectDetail   `bson:"projects"               json:"projects"`
	CreatedBy   string             `bson:"created_by"             json:"created_by"`
	CreatedTime int64              `bson:"created_time"           json:"created_time"`
	UpdateTime  int64              `bson:"update_time"            json:"update_time"`
	UpdateBy    string             `bson:"update_by"              json:"update_by,omitempty"`
}

type ProjectDetail struct {
	ProjectKey        string `bson:"project_key"                     json:"project_key"`
	ProjectName       string `bson:"project_name"                    json:"project_name"`
	ProjectDeployType string `bson:"project_deploy_type"             json:"project_deploy_type"`
}

func (ProjectGroup) TableName() string {
	return "project_group"
}
