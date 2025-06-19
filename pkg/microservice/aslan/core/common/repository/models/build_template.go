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

	"github.com/koderover/zadig/v2/pkg/types"
)

type BuildTemplate struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty"                 json:"id,omitempty"`
	Name                     string             `bson:"name"                          json:"name"`
	Team                     string             `bson:"team,omitempty"                json:"team,omitempty"`
	Source                   string             `bson:"source,omitempty"              json:"source,omitempty"`
	Timeout                  int                `bson:"timeout"                       json:"timeout"`
	UpdateTime               int64              `bson:"update_time"                   json:"update_time"`
	UpdateBy                 string             `bson:"update_by"                     json:"update_by"`
	PreBuild                 *PreBuild          `bson:"pre_build"                     json:"pre_build"`
	JenkinsBuild             *JenkinsBuild      `bson:"jenkins_build,omitempty"       json:"jenkins_build,omitempty"`
	ScriptType               types.ScriptType   `bson:"script_type"                   json:"script_type"`
	Scripts                  string             `bson:"scripts"                       json:"scripts"`
	PostBuild                *PostBuild         `bson:"post_build,omitempty"          json:"post_build"`
	SSHs                     []string           `bson:"sshs"                          json:"sshs"`
	PMDeployScripts          string             `bson:"pm_deploy_scripts"             json:"pm_deploy_scripts"`
	CacheEnable              bool               `bson:"cache_enable"                  json:"cache_enable"`
	CacheDirType             types.CacheDirType `bson:"cache_dir_type"                json:"cache_dir_type"`
	CacheUserDir             string             `bson:"cache_user_dir"                json:"cache_user_dir"`
	AdvancedSettingsModified bool               `bson:"advanced_setting_modified"     json:"advanced_setting_modified"`
	EnablePrivilegedMode     bool               `bson:"enable_privileged_mode"        json:"enable_privileged_mode"`
	Outputs                  []*Output          `bson:"outputs"                       json:"outputs"`
	Infrastructure           string             `bson:"infrastructure"                json:"infrastructure"`
	VmLabels                 []string           `bson:"vm_labels"                     json:"vm_labels"`
}

func (BuildTemplate) TableName() string {
	return "build_template"
}
