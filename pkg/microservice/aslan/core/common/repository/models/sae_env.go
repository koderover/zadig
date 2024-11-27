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

type SAEEnv struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProjectName   string             `bson:"project_name"              json:"project_name"`
	EnvName       string             `bson:"env_name"                  json:"env_name"`
	RegionID      string             `bson:"region_id"                 json:"region_id"`
	NamespaceName string             `bson:"namespace_name"            json:"namespace_name"`
	NamespaceID   string             `bson:"namespace_id"              json:"namespace_id"`
	Applications  []*SAEApplication  `bson:"-"                         json:"applications"`
	Production    bool               `bson:"production"                json:"production"`
	UpdateBy      string             `bson:"update_by"                 json:"update_by"`
	CreateTime    int64              `bson:"create_time"               json:"create_time"`
	UpdateTime    int64              `bson:"update_time"               json:"update_time"`
}

type SAETag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SAEApplication struct {
	AppName          string    `json:"app_name"`
	AppID            string    `json:"app_id"`
	Tags             []*SAETag `json:"tags"`
	ImageUrl         string    `json:"image_url"`
	PackageUrl       string    `json:"package_url"`
	Instances        int32     `json:"instances"`
	RunningInstances int32     `json:"running_instances"`
	Cpu              int32     `json:"cpu"`
	Mem              int32     `json:"mem"`
	ServiceName      string    `json:"service_name"`
	ServiceModule    string    `json:"service_module"`
}

func (SAEEnv) TableName() string {
	return "sae_env"
}
