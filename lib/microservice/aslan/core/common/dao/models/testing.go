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
	"github.com/koderover/zadig/lib/types"
)

type Testing struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty"            json:"id,omitempty"`
	Name            string              `bson:"name"                     json:"name"`
	ProductName     string              `bson:"product_name"             json:"product_name"`
	Desc            string              `bson:"desc"                     json:"desc"`
	Timeout         int                 `bson:"timeout"                  json:"timeout"`
	Team            string              `bson:"team"                     json:"team"`
	Repos           []*types.Repository `bson:"repos,omitempty"          json:"repos"`
	PreTest         *PreTest            `bson:"pre_test"                 json:"pre_test"`
	Scripts         string              `bson:"scripts"                  json:"scripts"`
	UpdateTime      int64               `bson:"update_time"              json:"update_time"`
	UpdateBy        string              `bson:"update_by"                json:"update_by"`
	TestResultPath  string              `bson:"test_result_path"         json:"test_result_path"`
	Threshold       int                 `bson:"threshold"                json:"threshold"`
	TestType        string              `bson:"test_type"                json:"test_type"`
	Caches          []string            `bson:"caches"                   json:"caches"`
	ArtifactPaths   []string            `bson:"artifact_paths,omitempty" json:"artifact_paths,omitempty"`
	TestCaseNum     int                 `bson:"-"                        json:"test_case_num,omitempty"`
	ExecuteNum      int                 `bson:"-"                        json:"execute_num,omitempty"`
	PassRate        float64             `bson:"-"                        json:"pass_rate,omitempty"`
	AvgDuration     float64             `bson:"-"                        json:"avg_duration,omitempty"`
	Workflows       []*Workflow         `bson:"-"                        json:"workflows,omitempty"`
	Schedules       *ScheduleCtrl       `bson:"schedules,omitempty"      json:"schedules,omitempty"`
	HookCtl         *TestingHookCtrl    `bson:"hook_ctl"                 json:"hook_ctl"`
	ScheduleEnabled bool                `bson:"schedule_enabled"         json:"-"`
}

type TestingHookCtrl struct {
	Enabled bool           `bson:"enabled" json:"enabled"`
	Items   []*TestingHook `bson:"items" json:"items"`
}

type TestingHook struct {
	AutoCancel bool          `bson:"auto_cancel" json:"auto_cancel"`
	MainRepo   MainHookRepo  `bson:"main_repo"   json:"main_repo"`
	TestArgs   *TestTaskArgs `bson:"test_args"   json:"test_args"`
}

// PreTest prepares an environment for a job
type PreTest struct {
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`
	// BuildOS defines job image OS, it supports 12.04, 14.04, 16.04
	BuildOS   string `bson:"build_os"                        json:"build_os"`
	ImageFrom string `bson:"image_from"                      json:"image_from"`
	ImageID   string `bson:"image_id"                        json:"image_id"`
	// ResReq defines job requested resources
	ResReq config.Request `bson:"res_req"                json:"res_req"`
	// Installs defines apps to be installed for build
	Installs []*Item `bson:"installs,omitempty"    json:"installs"`
	// Envs stores user defined env key val for build
	Envs []*KeyVal `bson:"envs,omitempty"              json:"envs"`
	// EnableProxy
	EnableProxy bool `bson:"enable_proxy"           json:"enable_proxy"`
}

func (Testing) TableName() string {
	return "module_testing"
}
