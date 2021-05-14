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

package task

import (
	"fmt"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type DistributeToS3 struct {
	TaskType       config.TaskType `bson:"type"                           json:"type"`
	Enabled        bool            `bson:"enabled"                        json:"enabled"`
	TaskStatus     config.Status   `bson:"status"                         json:"status"`
	PackageFile    string          `bson:"package_file"                   json:"package_file"`
	ProductName    string          `bson:"product_name"                   json:"product_name"`
	ServiceName    string          `bson:"service_name"                   json:"service_name"`
	RemoteFileKey  string          `bson:"remote_file_key"                json:"remote_file_key,omitempty"`
	Timeout        int             `bson:"timeout,omitempty"              json:"timeout,omitempty"`
	Error          string          `bson:"error,omitempty"                json:"error,omitempty"`
	StartTime      int64           `bson:"start_time,omitempty"           json:"start_time,omitempty"`
	EndTime        int64           `bson:"end_time,omitempty"             json:"end_time,omitempty"`
	DestStorageUrl string          `bson:"dest_storage_url"               json:"dest_storage_url"`
	//SrcStorageUrl  string `bson:"src_storage_url"                json:"src_storage_url"`
	S3StorageId string `bson:"s3_storage_id"                  json:"s3_storage_id"`
}

func (d *DistributeToS3) SetPackageFile(packageFile string) {
	if packageFile != "" {
		d.PackageFile = packageFile
	}
}

func (d *DistributeToS3) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(d, &task); err != nil {
		return nil, fmt.Errorf("convert DistributeToS3 task to interface error: %v", err)
	}
	return task, nil
}
