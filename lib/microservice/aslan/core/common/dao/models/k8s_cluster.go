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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type K8SCluster struct {
	ID           primitive.ObjectID      `json:"id,omitempty"                bson:"_id,omitempty"`
	Name         string                  `json:"name" bson:"name"`
	Tags         []string                `json:"tags" bson:"tags"`
	Description  string                  `json:"description" bson:"description"`
	Namespace    string                  `json:"namespace" bson:"namespace"`
	Info         *K8SClusterInfo         `json:"info,omitempty" bson:"info,omitempty"`
	Status       config.K8SClusterStatus `json:"status" bson:"status"`
	Error        string                  `json:"error" bson:"error"`
	Yaml         string                  `json:"yaml" bson:"yaml"`
	Production   bool                    `json:"production" bson:"production"`
	CreatedAt    int64                   `json:"createdAt" bson:"createdAt"`
	CreatedBy    string                  `json:"createdBy" bson:"createdBy"`
	Disconnected bool                    `json:"-" bson:"disconnected"`
	Token        string                  `json:"token" bson:"-"`
}

type K8SClusterInfo struct {
	Nodes   int    `json:"nodes" bson:"nodes"`
	Version string `json:"version" bson:"version"`
	CPU     string `json:"cpu" bson:"cpu"`
	Memory  string `json:"memory" json:"memory"`
}

func (K8SCluster) TableName() string {
	return "k8s_cluster"
}
