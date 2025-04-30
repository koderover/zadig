/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

import (
	"github.com/koderover/zadig/v2/pkg/util"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type Scanning struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string             `bson:"name"          json:"name"`
	TemplateID  string             `bson:"template_id"   json:"template_id"`
	ProjectName string             `bson:"project_name"  json:"project_name"`
	Description string             `bson:"description"   json:"description"`
	ScannerType string             `bson:"scanner_type"  json:"scanner_type"`
	// EnableScanner indicates whether user uses sonar scanner instead of the script
	EnableScanner  bool                `bson:"enable_scanner" json:"enable_scanner"`
	ImageID        string              `bson:"image_id"      json:"image_id"`
	SonarID        string              `bson:"sonar_id"      json:"sonar_id"`
	Repos          []*types.Repository `bson:"repos"         json:"repos"`
	Installs       []*Item             `bson:"installs"      json:"installs"`
	Infrastructure string              `bson:"infrastructure"           json:"infrastructure"`
	VMLabels       []string            `bson:"vm_labels"                json:"vm_labels"`
	// Parameter is for sonarQube type only
	Parameter string `bson:"parameter" json:"parameter"`
	// Envs is the user defined key/values
	Envs KeyValList `bson:"envs" json:"envs"`
	// Script is for other type only
	ScriptType       types.ScriptType         `bson:"script_type"           json:"script_type"`
	Script           string                   `bson:"script"                json:"script"`
	AdvancedSetting  *ScanningAdvancedSetting `bson:"advanced_setting"      json:"advanced_setting"`
	CheckQualityGate bool                     `bson:"check_quality_gate"    json:"check_quality_gate"`
	Outputs          []*Output                `bson:"outputs"               json:"outputs"`

	CreatedAt int64  `bson:"created_at" json:"created_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
	UpdatedBy string `bson:"updated_by" json:"updated_by"`
}

func (Scanning) TableName() string {
	return "scanning"
}

type ScanningAdvancedSetting struct {
	ArtifactPaths     []string              `bson:"artifact_paths"    json:"artifact_paths"`
	ClusterID         string                `bson:"cluster_id"        json:"cluster_id"`
	ClusterSource     string                `bson:"cluster_source"    json:"cluster_source"`
	StrategyID        string                `bson:"strategy_id"       json:"strategy_id"`
	Timeout           int64                 `bson:"timeout"           json:"timeout"`
	ResReq            setting.Request       `bson:"res_req"           json:"res_req"`
	ResReqSpec        setting.RequestSpec   `bson:"res_req_spec"      json:"res_req_spec"`
	HookCtl           *ScanningHookCtl      `bson:"hook_ctl"          json:"hook_ctl"`
	NotifyCtls        []*NotifyCtl          `bson:"notify_ctls"       json:"notify_ctls"`
	Cache             *ScanningCacheSetting `bson:"cache"             json:"cache"`
	ConcurrencyLimit  int                   `bson:"concurrency_limit" json:"concurrency_limit"`
	CustomAnnotations []*util.KeyValue      `bson:"custom_annotations" json:"custom_annotations"`
	CustomLabels      []*util.KeyValue      `bson:"custom_labels"      json:"custom_labels"`
}

type ScanningHookCtl struct {
	Enabled bool            `bson:"enabled"      json:"enabled"`
	Items   []*ScanningHook `bson:"items"        json:"items"`
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
	IsManual     bool                   `bson:"is_manual"     json:"is_manual"`
	AutoCancel   bool                   `bson:"auto_cancel"   json:"auto_cancel"`
}

type SonarInfo struct {
	ServerAddress string `bson:"server_address" json:"server_address"`
	Token         string `bson:"token"          json:"token"`
}

type ScanningCacheSetting struct {
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
}
