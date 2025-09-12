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

import "go.mongodb.org/mongo-driver/bson/primitive"

type Plugin struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"  json:"id,omitempty"`
	Name        string             `bson:"name"           json:"name"`
	Identifier  string             `bson:"identifier"     json:"identifier"`
	Index       int                `bson:"index"          json:"index"`
	Type        string             `bson:"type"           json:"type"` // page or tab
	Description string             `bson:"description"    json:"description"`
	Route       string             `bson:"route"          json:"route"`
	Filters     []*PluginFilter    `bson:"filters,omitempty" json:"filters,omitempty"`
	StorageID   string             `bson:"storage_id"     json:"storage_id"`
	FilePath    string             `bson:"file_path"      json:"file_path"`
	FileName    string             `bson:"file_name"      json:"file_name"`
	FileSize    int64              `bson:"file_size"      json:"file_size"`
	FileHash    string             `bson:"file_hash"      json:"file_hash"`
	Enabled     bool               `bson:"enabled"        json:"enabled"`

	CreateTime int64  `bson:"create_time"    json:"create_time"`
	UpdateTime int64  `bson:"update_time"    json:"update_time"`
	CreateBy   string `bson:"create_by"      json:"create_by"`
	UpdateBy   string `bson:"update_by"      json:"update_by"`
}

func (Plugin) TableName() string { return "plugin" }

type PluginFilter struct {
	Field string      `bson:"field"  json:"field"`
	Verb  string      `bson:"verb"   json:"verb"`
	Value interface{} `bson:"value"  json:"value"`
}
