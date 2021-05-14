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

type Config struct {
	ConfigName  string       `bson:"config_name"               json:"config_name"`
	ServiceName string       `bson:"service_name"              json:"service_name"`
	Type        string       `bson:"type"                      json:"type"`
	Team        string       `bson:"team"                      json:"team"`
	Revision    int64        `bson:"revision"                  json:"revision"`
	CreateTime  int64        `bson:"create_time"               json:"create_time"`
	CreateBy    string       `bson:"create_by"                 json:"create_by"`
	Descritpion string       `bson:"description,omitempty"     json:"description,omitempty"`
	ConfigData  []ConfigData `bson:"config_data"               json:"config_data"`
	RenderData  []RenderData `bson:"render_data"               json:"render_data"`
}

// ConfigData ...
type ConfigData struct {
	Key   string `bson:"key"              json:"key"`
	Value string `bson:"value"            json:"value"`
}

// RenderData

type RenderData struct {
	Name         string        `bson:"name"                  json:"name"`
	IsCredential bool          `bson:"is_credential"         json:"is_credential"`
	KV           []RenderValue `bson:"kv"                    json:"kv"`
}

// RenderValue ...
type RenderValue struct {
	Product string `bson:"product"         json:"product"`
	Value   string `bson:"value"           json:"value"`
}

// ConfigPipeResp ...
type ConfigPipeResp struct {
	Config struct {
		ConfigName  string `bson:"config_name"         json:"config_name"`
		ServiceName string `bson:"service_name"        json:"service_name"`
		ProductName string `bson:"product_name"        json:"product_name"`
	} `bson:"_id"      json:"config"`
	Revision int64 `bson:"revision"                    json:"revision"`
}

func (Config) TableName() string {
	return "template_config"
}
