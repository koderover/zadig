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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Build struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	Name        string             `bson:"name"                         json:"name"`
	ProductName string             `bson:"product_name"                 json:"product_name"`
	PreBuild    *PreBuild          `bson:"pre_build"                    json:"pre_build"`
	Timeout     int                `bson:"timeout"                      json:"timeout"`
	// TODO: Deprecated.
	Caches []string `bson:"caches"                        json:"caches"`

	// New since V1.10.0.
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
	// New since V1.10.0. Only to tell the webpage should the advanced settings be displayed
	AdvancedSettingsModified bool          `bson:"advanced_setting_modified"     json:"advanced_setting_modified"`
	JenkinsBuild             *JenkinsBuild `bson:"jenkins_build,omitempty"       json:"jenkins_build,omitempty"`
}

type JenkinsBuild struct {
	JenkinsID         string                     `bson:"jenkins_id"          json:"jenkins_id"`
	JobName           string                     `bson:"job_name"            json:"job_name"`
	JenkinsBuildParam []*types.JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
}

// PreBuild prepares an environment for a job
type PreBuild struct {
	ClusterID string          `bson:"cluster_id"             json:"cluster_id"`
	ResReq    setting.Request `bson:"res_req"                json:"res_req"`
	BuildOS   string          `bson:"build_os"                      json:"build_os"`
	ImageID   string          `bson:"image_id"                      json:"image_id"`
	// TODO: Deprecated.
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`
}

func (Build) TableName() string {
	return "module_build"
}
