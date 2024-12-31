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
	"github.com/koderover/zadig/v2/pkg/util"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type Testing struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty"            json:"id,omitempty"`
	Name           string              `bson:"name"                     json:"name"`
	ProductName    string              `bson:"product_name"             json:"product_name"`
	Desc           string              `bson:"desc"                     json:"desc"`
	Timeout        int                 `bson:"timeout"                  json:"timeout"`
	Team           string              `bson:"team"                     json:"team"`
	Infrastructure string              `bson:"infrastructure"           json:"infrastructure"`
	VMLabels       []string            `bson:"vm_labels"                json:"vm_labels"`
	Repos          []*types.Repository `bson:"repos"                    json:"repos"`
	PreTest        *PreTest            `bson:"pre_test"                 json:"pre_test"`
	PostTest       *PostTest           `bson:"post_test"                json:"post_test"`
	ScriptType     types.ScriptType    `bson:"script_type"              json:"script_type"`
	Scripts        string              `bson:"scripts"                  json:"scripts"`
	UpdateTime     int64               `bson:"update_time"              json:"update_time"`
	UpdateBy       string              `bson:"update_by"                json:"update_by"`
	// Junit 测试报告
	TestResultPath string `bson:"test_result_path"         json:"test_result_path"`
	// html 测试报告
	TestReportPath string `bson:"test_report_path"         json:"test_report_path"`
	Threshold      int    `bson:"threshold"                json:"threshold"`
	TestType       string `bson:"test_type"                json:"test_type"`

	// TODO: Deprecated.
	Caches []string `bson:"caches"                   json:"caches"`

	ArtifactPaths []string         `bson:"artifact_paths"           json:"artifact_paths,omitempty"`
	TestCaseNum   int              `bson:"-"                        json:"test_case_num,omitempty"`
	ExecuteNum    int              `bson:"-"                        json:"execute_num,omitempty"`
	PassRate      float64          `bson:"-"                        json:"pass_rate,omitempty"`
	AvgDuration   float64          `bson:"-"                        json:"avg_duration,omitempty"`
	Workflows     []*Workflow      `bson:"-"                        json:"workflows,omitempty"`
	Schedules     *ScheduleCtrl    `bson:"schedules,omitempty"      json:"schedules,omitempty"`
	HookCtl       *TestingHookCtrl `bson:"hook_ctl"                 json:"hook_ctl"`
	// TODO: Deprecated.
	NotifyCtl *NotifyCtl `bson:"notify_ctl,omitempty"     json:"notify_ctl,omitempty"`
	// New since V1.12.0.
	NotifyCtls      []*NotifyCtl `bson:"notify_ctls"     json:"notify_ctls"`
	ScheduleEnabled bool         `bson:"schedule_enabled"         json:"-"`

	// New since V1.10.0.
	CacheEnable  bool               `bson:"cache_enable"              json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"            json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"            json:"cache_user_dir"`
	// New since V1.10.0. Only to tell the webpage should the advanced settings be displayed
	AdvancedSettingsModified bool      `bson:"advanced_setting_modified" json:"advanced_setting_modified"`
	Outputs                  []*Output `bson:"outputs"                   json:"outputs"`
}

type TestingHookCtrl struct {
	Enabled bool           `bson:"enabled" json:"enabled"`
	Items   []*TestingHook `bson:"items" json:"items"`
}

type TestingHook struct {
	AutoCancel bool          `bson:"auto_cancel" json:"auto_cancel"`
	IsManual   bool          `bson:"is_manual"   json:"is_manual"`
	MainRepo   *MainHookRepo `bson:"main_repo"   json:"main_repo"`
	TestArgs   *TestTaskArgs `bson:"test_args"   json:"test_args"`
}

// PreTest prepares an environment for a job
type PreTest struct {
	// TODO: Deprecated.
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`

	// BuildOS defines job image OS, it supports 18.04 and 20.04
	BuildOS   string `bson:"build_os"                        json:"build_os"`
	ImageFrom string `bson:"image_from"                      json:"image_from"`
	ImageID   string `bson:"image_id"                        json:"image_id"`
	// ResReq defines job requested resources
	ResReq     setting.Request     `bson:"res_req"                json:"res_req"`
	ResReqSpec setting.RequestSpec `bson:"res_req_spec"           json:"res_req_spec"`
	// Installs defines apps to be installed for build
	Installs []*Item `bson:"installs,omitempty"    json:"installs"`
	// Envs stores user defined env key val for build
	Envs []*KeyVal `bson:"envs,omitempty"              json:"envs"`
	// EnableProxy
	EnableProxy      bool   `bson:"enable_proxy"           json:"enable_proxy"`
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"`
	ClusterSource    string `bson:"cluster_source"         json:"cluster_source"`
	StrategyID       string `bson:"strategy_id"            json:"strategy_id"`
	ConcurrencyLimit int    `bson:"concurrency_limit"      json:"concurrency_limit"`
	// TODO: Deprecated.
	Namespace string `bson:"namespace"              json:"namespace"`

	CustomAnnotations []*util.KeyValue `bson:"custom_annotations"        json:"custom_annotations"`
	CustomLabels      []*util.KeyValue `bson:"custom_labels"             json:"custom_labels"`
}

type PostTest struct {
	ObjectStorageUpload *ObjectStorageUpload `bson:"object_storage_upload,omitempty" json:"object_storage_upload,omitempty"`
}

func (Testing) TableName() string {
	return "module_testing"
}
