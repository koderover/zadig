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

type DindClean struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"       json:"_id,omitempty"`
	Name           string             `bson:"name"                json:"-"`
	Status         string             `bson:"status"              json:"status"`
	UpdateTime     int64              `bson:"update_time"         json:"update_time"`
	DindCleanInfos []*DindCleanInfo   `bson:"dind_clean_infos"    json:"dind_clean_infos"`
}

type DindCleanInfo struct {
	StartTime    int64  `bson:"start_time"                  json:"start_time"`
	PodName      string `bson:"pod_name"                    json:"pod_name"`
	CleanInfo    string `bson:"clean_info"                  json:"clean_info"`
	EndTime      int64  `bson:"end_time"                    json:"end_time"`
	ErrorMessage string `bson:"error_message,omitempty"     json:"error_message"`
}

func (DindClean) TableName() string {
	return "dind_clean"
}
