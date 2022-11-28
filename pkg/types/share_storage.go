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

package types

import (
	"fmt"
	"path"
)

const (
	pathPrefix = "zadig-share-storage"
)

type ShareStorage struct {
	MediumType    MediumType    `json:"medium_type"       bson:"medium_type"           yaml:"medium_type"`
	NFSProperties NFSProperties `json:"nfs_properties"    bson:"nfs_properties"        yaml:"nfs_properties"`
}

func GetShareStorageSubPath(workflowName, storageName string, taskID int64) string {
	return path.Join(pathPrefix, fmt.Sprintf("%s-%d", workflowName, taskID), storageName)
}

func GetShareStorageSubPathPrefix(workflowName string, taskID int64) string {
	return path.Join(pathPrefix, fmt.Sprintf("%s-%d", workflowName, taskID))
}
