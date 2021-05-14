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

type DeliveryDistribute struct {
	ID             primitive.ObjectID    `bson:"_id,omitempty"                json:"id,omitempty"`
	ReleaseID      primitive.ObjectID    `bson:"release_id"             json:"releaseId"`
	ServiceName    string                `bson:"service_name"           json:"serviceName"`
	DistributeType config.DistributeType `bson:"distribute_type"        json:"distributeType"`
	RegistryName   string                `bson:"registry_name"          json:"registryName"`
	Namespace      string                `bson:"namespace"              json:"namespace"`
	PackageFile    string                `bson:"package_file"           json:"packageFile"`
	RemoteFileKey  string                `bson:"remote_file_key"        json:"remoteFileKey"`
	DestStorageUrl string                `bson:"dest_storage_url"       json:"destStorageUrl"`
	SrcStorageUrl  string                `bson:"src_storage_url"        json:"srcStorageUrl"`
	StartTime      int64                 `bson:"start_time,omitempty"   json:"start_time,omitempty"`
	EndTime        int64                 `bson:"end_time,omitempty"     json:"end_time,omitempty"`
	CreatedAt      int64                 `bson:"created_at"             json:"created_at"`
	DeletedAt      int64                 `bson:"deleted_at"             json:"deleted_at"`
}

func (DeliveryDistribute) TableName() string {
	return "delivery_distribute"
}
