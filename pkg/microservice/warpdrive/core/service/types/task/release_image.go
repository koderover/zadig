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

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
)

type ReleaseImageItem struct {
	RepoID string `bson:"id" json:"id"`
	Image  string `bson:"image" json:"image"`
}

// ReleaseImage 线上镜像分发任务
type ReleaseImage struct {
	TaskType     config.TaskType `bson:"type"                           json:"type"`
	Enabled      bool            `bson:"enabled"                        json:"enabled"`
	TaskStatus   config.Status   `bson:"status"                         json:"status"`
	ImageTest    string          `bson:"image_test"                     json:"image_test"`
	ImageRelease string          `bson:"image_release"                  json:"image_release"`
	ImageRepo    string          `bson:"image_repo"                     json:"image_repo"`
	Timeout      int             `bson:"timeout,omitempty"              json:"timeout,omitempty"`
	Error        string          `bson:"error,omitempty"                json:"error,omitempty"`
	StartTime    int64           `bson:"start_time,omitempty"           json:"start_time,omitempty"`
	EndTime      int64           `bson:"end_time,omitempty"             json:"end_time,omitempty"`
	LogFile      string          `bson:"log_file"                       json:"log_file"`

	// destinations to distribute images
	Releases []RepoImage `bson:"releases"                 json:"releases"`

	// New Version field
	ProductName    string            `bson:"product_name"    json:"product_name"`
	SourceImage    string            `bson:"source_image"    json:"source_image"`
	DistributeInfo []*DistributeInfo `bson:"distribute_info" json:"distribute_info"`
}

// DistributeInfo will be convert into yaml, adding yaml
type DistributeInfo struct {
	Image               string `json:"image"                 yaml:"image"`
	ReleaseName         string `json:"release_name"          yaml:"release_name"`
	DistributeStartTime int64  `json:"distribute_start_time" yaml:"distribute_start_time"`
	DistributeEndTime   int64  `json:"distribute_end_time"   yaml:"distribute_end_time"`
	DistributeStatus    string `json:"distribute_status"     yaml:"distribute_status"`
	DeployEnabled       bool   `json:"deploy_enabled"        yaml:"deploy_enabled"`
	DeployEnv           string `json:"deploy_env"            yaml:"deploy_env"`
	DeployServiceType   string `json:"deploy_service_type"   yaml:"deploy_service_type"`
	DeployContainerName string `json:"deploy_container_name" yaml:"deploy_container_name"`
	DeployServiceName   string `json:"deploy_service_name"   yaml:"deploy_service_name"`
	DeployClusterID     string `json:"deploy_cluster_id"     yaml:"deploy_cluster_id"`
	DeployNamespace     string `json:"deploy_namespace"      yaml:"deploy_namespace"`
	DeployStartTime     int64  `json:"deploy_start_time"     yaml:"deploy_start_time"`
	DeployEndTime       int64  `json:"deploy_end_time"       yaml:"deploy_end_time"`
	DeployStatus        string `json:"deploy_status"         yaml:"deploy_status"`
	RepoID              string `json:"repo_id"               yaml:"repo_id"`

	//RepoAK and RepoSK is used for docker login, it is determined runtime, not passed by message so no json is required
	RepoAK string `json:"-"  yaml:"repo_ak"`
	RepoSK string `json:"-"  yaml:"repo_sk"`
}

type RepoImage struct {
	RepoID    string `json:"repo_id" bson:"repo_id"`
	Name      string `json:"name" bson:"name" yaml:"name"`
	Username  string `json:"-" yaml:"username"`
	Password  string `json:"-" yaml:"password"`
	Host      string `json:"host" yaml:"host"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

func (ri *ReleaseImage) SetImage(image string) {
	if image != "" {
		ri.ImageTest = image
	}
}

func (ri *ReleaseImage) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(ri, &task); err != nil {
		return nil, fmt.Errorf("convert ReleaseImageTask to interface error: %v", err)
	}
	return task, nil
}
