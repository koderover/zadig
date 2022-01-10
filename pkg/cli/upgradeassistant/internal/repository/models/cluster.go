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

import (
	"github.com/koderover/zadig/pkg/setting"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type K8SCluster struct {
	ID     primitive.ObjectID       `json:"id,omitempty"              bson:"_id,omitempty"`
	Name   string                   `json:"name"                      bson:"name"`
	Status setting.K8SClusterStatus `json:"status"                    bson:"status"`
	Local  bool                     `json:"local"                     bson:"local"`
}

func (K8SCluster) TableName() string {
	return "k8s_cluster"
}
