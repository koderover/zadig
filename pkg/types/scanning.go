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
	Timeout    int64               `bson:"timeout"`
	ResReq     setting.Request     `json:"res_req"`
	ResReqSpec setting.RequestSpec `json:"res_req_spec"`
	HookCtl    *ScanningHookCtl    `json:"hook_ctl"`
}

type ScanningHookCtl struct {
	Enabled bool            `json:"enabled"`
	Items   []*ScanningHook `json:"items"`
}

type ScanningHook struct {
	CodehostID   int                    `json:"codehost_id"`
	Source       string                 `json:"source"`
	RepoOwner    string                 `json:"repo_owner"`
	RepoName     string                 `json:"repo_name"`
	Branch       string                 `json:"branch"`
	Events       []config.HookEventType `json:"events"`
	MatchFolders []string               `json:"match_folders"`
}
