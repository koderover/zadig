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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Testing struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"            json:"id,omitempty"`
	PreTest    *PreTest           `bson:"pre_test"                 json:"pre_test"`
	Timeout    int                `bson:"timeout"                  json:"timeout"`
	HookCtl    *TestingHookCtrl   `bson:"hook_ctl"                 json:"hook_ctl"`
	NotifyCtl  *NotifyCtl         `bson:"notify_ctl,omitempty"     json:"notify_ctl,omitempty"`
	NotifyCtls []*NotifyCtl       `bson:"notify_ctls"              json:"notify_ctls"`

	Schedules *ScheduleCtrl `bson:"schedules,omitempty"      json:"schedules,omitempty"`

	ArtifactPaths []string `bson:"artifact_paths,omitempty" json:"artifact_paths,omitempty"`

	// TODO: Deprecated.
	Caches []string `bson:"caches"                   json:"caches"`

	// New since V1.10.0.
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
	// New since V1.10.0. Only to tell the webpage should the advanced settings be displayed
	AdvancedSettingsModified bool `bson:"advanced_setting_modified" json:"advanced_setting_modified"`
}

// PreTest prepares an environment for a job
type PreTest struct {
	ClusterID string          `bson:"cluster_id"             json:"cluster_id"`
	ResReq    setting.Request `bson:"res_req"                json:"res_req"`
	BuildOS   string          `bson:"build_os"                        json:"build_os"`
	ImageID   string          `bson:"image_id"                        json:"image_id"`
	// TODO: Deprecated.
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`
}

type TestingHookCtrl struct {
	Enabled bool           `bson:"enabled" json:"enabled"`
	Items   []*TestingHook `bson:"items" json:"items"`
}

type ScheduleCtrl struct {
	Enabled bool `bson:"enabled"    json:"enabled"`
	//Items   []*Schedule `bson:"items"      json:"items"`
}

type NotifyCtl struct {
	Enabled         bool     `bson:"enabled"                          json:"enabled"`
	WebHookType     string   `bson:"webhook_type"                     json:"webhook_type"`
	WeChatWebHook   string   `bson:"weChat_webHook,omitempty"         json:"weChat_webHook,omitempty"`
	DingDingWebHook string   `bson:"dingding_webhook,omitempty"       json:"dingding_webhook,omitempty"`
	FeiShuWebHook   string   `bson:"feishu_webhook,omitempty"         json:"feishu_webhook,omitempty"`
	AtMobiles       []string `bson:"at_mobiles,omitempty"             json:"at_mobiles,omitempty"`
	IsAtAll         bool     `bson:"is_at_all,omitempty"              json:"is_at_all,omitempty"`
	NotifyTypes     []string `bson:"notify_type"                      json:"notify_type"`
}

type TestingHook struct {
	AutoCancel bool          `bson:"auto_cancel" json:"auto_cancel"`
	MainRepo   *MainHookRepo `bson:"main_repo"   json:"main_repo"`
	TestArgs   *TestTaskArgs `bson:"test_args"   json:"test_args"`
}

type MainHookRepo struct {
	Name         string                 `bson:"name,omitempty"            json:"name,omitempty"`
	Description  string                 `bson:"description,omitempty"     json:"description,omitempty"`
	Source       string                 `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner    string                 `bson:"repo_owner"                json:"repo_owner"`
	RepoName     string                 `bson:"repo_name"                 json:"repo_name"`
	Branch       string                 `bson:"branch"                    json:"branch"`
	Tag          string                 `bson:"tag"                       json:"tag"`
	Committer    string                 `bson:"committer"                 json:"committer"`
	MatchFolders []string               `bson:"match_folders"             json:"match_folders,omitempty"`
	CodehostID   int                    `bson:"codehost_id"               json:"codehost_id"`
	Events       []config.HookEventType `bson:"events"                    json:"events"`
	Label        string                 `bson:"label"                     json:"label"`
	Revision     string                 `bson:"revision"                  json:"revision"`
	IsRegular    bool                   `bson:"is_regular"                json:"is_regular"`
}

type TestTaskArgs struct {
	ProductName     string `bson:"product_name"            json:"product_name"`
	TestName        string `bson:"test_name"               json:"test_name"`
	TestTaskCreator string `bson:"test_task_creator"       json:"test_task_creator"`
	NotificationID  string `bson:"notification_id"         json:"notification_id"`
	ReqID           string `bson:"req_id"                  json:"req_id"`
	// webhook触发测试任务时，触发任务的repo、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
}

func (Testing) TableName() string {
	return "module_testing"
}
