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

type DeliveryDeploy struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	ReleaseID       primitive.ObjectID `bson:"release_id"                  json:"releaseId"`
	ServiceName     string             `bson:"service_name"                json:"serviceName"`
	ContainerName   string             `bson:"container_name"              json:"containerName"`
	Image           string             `bson:"image,omitempty"             json:"image,omitempty"`
	RegistryID      string             `bson:"registry_id,omitempty"       json:"registry_id,omitempty"`
	YamlContents    []string           `bson:"yaml_contents"               json:"yamlContents"`
	Envs            []EnvObjects       `bson:"envs"                        json:"envs"`
	OrderedServices [][]string         `bson:"ordered_services"            json:"orderedServices"`
	StartTime       int64              `bson:"start_time,omitempty"        json:"start_time,omitempty"`
	EndTime         int64              `bson:"end_time,omitempty"          json:"end_time,omitempty"`
	CreatedAt       int64              `bson:"created_at"                  json:"created_at"`
	DeletedAt       int64              `bson:"deleted_at"                  json:"deleted_at"`
}

type EnvObjects struct {
	Key   string `bson:"key"               json:"key"`
	Value string `bson:"value"             json:"value"`
}

func (DeliveryDeploy) TableName() string {
	return "delivery_deploy"
}
