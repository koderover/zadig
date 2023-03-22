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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
)

type ScanningAdvancedSetting struct {
	ClusterID  string              `bson:"cluster_id"   json:"cluster_id"`
	Timeout    int64               `bson:"timeout"      json:"timeout"`
	ResReq     setting.Request     `bson:"res_req"      json:"res_req"`
	ResReqSpec setting.RequestSpec `bson:"res_req_spec" json:"res_req_spec"`
	HookCtl    *ScanningHookCtl    `bson:"hook_ctl"     json:"hook_ctl"`
}

type ScanningHookCtl struct {
	Enabled bool            `bson:"enabled" json:"enabled"`
	Items   []*ScanningHook `bson:"items"   json:"items"`
}

type ScanningHook struct {
	CodehostID   int                    `bson:"codehost_id"   json:"codehost_id"`
	Source       string                 `bson:"source"        json:"source"`
	RepoOwner    string                 `bson:"repo_owner"    json:"repo_owner"`
	RepoName     string                 `bson:"repo_name"     json:"repo_name"`
	Branch       string                 `bson:"branch"        json:"branch"`
	Events       []config.HookEventType `bson:"events"        json:"events"`
	MatchFolders []string               `bson:"match_folders" json:"match_folders"`
	IsRegular    bool                   `bson:"is_regular"    json:"is_regular"`
}

type SonarInfo struct {
	ServerAddress string `bson:"server_address" json:"server_address"`
	Token         string `bson:"token"          json:"token"`
}
