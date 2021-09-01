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

type WorkLoadStat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	ClusterID string             `bson:"cluster_id"            json:"cluster_id"`
	Namespace string             `bson:"namespace"             json:"namespace"`
	Workloads []WorkLoad         `bson:"workloads"             json:"workloads"`
}

type WorkLoad struct {
	OccupyBy string `bson:"occupy_by"         json:"occupy_by"`
	Name     string `bson:"name"              json:"name"`
}

func (WorkLoadStat) TableName() string {
	return "workload_stat"
}

type SaveK8sWorkloadsArgs struct {
	WorkLoads []string `bson:"workLoads"         json:"workLoads"`
	EnvName   string   `bson:"env_name"         json:"env_name"`
	ClusterID string   `bson:"cluster_id"         json:"cluster_id"`
	Namespace string   `bson:"namespace"         json:"namespace"`
}
