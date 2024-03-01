/*
Copyright 2021 The KodeRover Authors.

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

type DeliverySecurity struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	ImageID       string             `bson:"image_id"              json:"imageId"`
	ImageName     string             `bson:"image_name"            json:"imageName"`
	LayerID       string             `bson:"layer_id"              json:"layerId"`
	Vulnerability Vulnerability      `bson:"vulnerability"         json:"vulnerability"`
	Feature       Feature            `bson:"feature"               json:"feature"`
	Severity      string             `bson:"severity"              json:"severity"`
	CreatedAt     int64              `bson:"created_at"            json:"created_at"`
	DeletedAt     int64              `bson:"deleted_at"            json:"deleted_at"`
}

type Vulnerability struct {
	Name          string                 `json:"name,omitempty"`
	NamespaceName string                 `json:"namespaceName,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Link          string                 `json:"link,omitempty"`
	Severity      string                 `json:"severity,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	FixedBy       string                 `json:"fixedBy,omitempty"`
	FixedIn       []Feature              `json:"fixedIn,omitempty"`
}

type Feature struct {
	Name            string          `json:"name,omitempty"`
	NamespaceName   string          `json:"namespaceName,omitempty"`
	VersionFormat   string          `json:"versionFormat,omitempty"`
	Version         string          `json:"version,omitempty"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`
	AddedBy         string          `json:"addedBy,omitempty"`
}

func (DeliverySecurity) TableName() string {
	return "delivery_security"
}
