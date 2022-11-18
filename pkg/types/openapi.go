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

type OpenAPIRepoInput struct {
	CodeHostName  string `json:"codehost_name"`
	RepoNamespace string `json:"repo_namespace"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PR            int    `json:"pr"`
	PRs           []int  `json:"prs"`
	RemoteName    string `json:"remote_name"`
	CheckoutPath  string `json:"checkout_path"`
	SubModules    bool   `json:"submodules"`
}

type OpenAPIWebhookConfigDetail struct {
	CodeHostName  string                 `json:"codehost_name"`
	RepoNamespace string                 `json:"repo_namespace"`
	RepoName      string                 `json:"repo_name"`
	Branch        string                 `json:"branch"`
	Events        []config.HookEventType `json:"events"`
	MatchFolders  []string               `json:"match_folders"`
}

type OpenAPIAdvancedSetting struct {
	ClusterName string                 `json:"cluster_name"`
	Timeout     int64                  `json:"timeout"`
	Spec        setting.RequestSpec    `json:"resource_spec"`
	Webhooks    *OpenAPIWebhookSetting `json:"webhooks"`
	// Cache settings is for build only for now, remove this line if there are further changes
	CacheSetting *OpenAPICacheSetting `json:"cache_setting"`
}

type OpenAPIWebhookSetting struct {
	Enabled  bool                          `json:"enabled"`
	HookList []*OpenAPIWebhookConfigDetail `json:"hook_list"`
}

type OpenAPICacheSetting struct {
	Enabled  bool   `json:"enabled"`
	CacheDir string `json:"cache_dir"`
}

type OpenAPIServiceBuildArgs struct {
	ServiceModule string              `json:"service_module"`
	ServiceName   string              `json:"service_name"`
	RepoInfo      []*OpenAPIRepoInput `json:"repo_info"`
	Inputs        []*KV               `json:"inputs"`
}

type KV struct {
	Key          string `json:"key"`
	Value        string `json:"value"`
	Type         string `json:"type,omitempty"`
	IsCredential bool   `json:"is_credential,omitempty"`
}
