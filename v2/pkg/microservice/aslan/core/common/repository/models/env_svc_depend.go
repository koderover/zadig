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

import "go.mongodb.org/mongo-driver/bson/primitive"

type EnvSvcDepend struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName   string             `bson:"product_name"              json:"product_name"`
	Namespace     string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	EnvName       string             `bson:"env_name"                  json:"env_name"`
	ServiceName   string             `bson:"service_name"              json:"service_name"`
	ServiceModule string             `bson:"service_module"            json:"service_module"`
	WorkloadType  string             `bson:"workload_type"             json:"workload_type"`
	Pvcs          []string           `bson:"pvcs"                      json:"pvcs"`
	ConfigMaps    []string           `bson:"configmaps"                json:"configmaps"`
	Secrets       []string           `bson:"secrets"                   json:"secrets"`
	CreateTime    int64              `bson:"create_time"               json:"create_time"`
}

func (EnvSvcDepend) TableName() string {
	return "env_svc_depend"
}
