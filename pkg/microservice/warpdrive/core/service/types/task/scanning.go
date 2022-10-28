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

package task

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Scanning struct {
	TaskType   config.TaskType     `bson:"type"            json:"type"`
	Status     config.Status       `bson:"status"          json:"status,omitempty"`
	ScanningID string              `bson:"scanning_id"     json:"scanning_id"`
	Name       string              `bson:"name"            json:"name"`
	Error      string              `bson:"error,omitempty" json:"error,omitempty"`
	ImageInfo  string              `bson:"image_info"      json:"image_info"`
	SonarInfo  *types.SonarInfo    `bson:"sonar_info"      json:"sonar_info"`
	Repos      []*types.Repository `bson:"repos"           json:"repos"`
	Proxy      *models.Proxy       `bson:"proxy"           json:"proxy"`
	ClusterID  string              `bson:"cluster_id"      json:"cluster_id"`
	// ResReq defines job requested resources
	ResReq     setting.Request      `bson:"res_req"       json:"res_req"`
	ResReqSpec setting.RequestSpec  `bson:"res_req_spec"  json:"res_req_spec"`
	Timeout    int64                `bson:"timeout"       json:"timeout"`
	Registries []*RegistryNamespace `bson:"-"             json:"registries"`
	// Parameter is for sonarQube type only
	Parameter string `bson:"parameter" json:"parameter"`
	// Script is for other type only
	Script           string     `bson:"script"                json:"script"`
	PreScript        string     `bson:"pre_script"            json:"pre_script"`
	CheckQualityGate bool       `bson:"check_quality_gate"    json:"check_quality_gate"`
	InstallCtx       []*Install `bson:"-"                     json:"install_ctx"`
}
