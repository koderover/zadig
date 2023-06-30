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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Scanning struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string              `bson:"name"          json:"name"`
	ProjectName string              `bson:"project_name"  json:"project_name"`
	Description string              `bson:"description"   json:"description"`
	ScannerType string              `bson:"scanner_type"  json:"scanner_type"`
	ImageID     string              `bson:"image_id"      json:"image_id"`
	SonarID     string              `bson:"sonar_id"      json:"sonar_id"`
	Repos       []*types.Repository `bson:"repos"         json:"repos"`
	Installs    []*Item             `bson:"installs"      json:"installs"`
	PreScript   string              `bson:"pre_script"    json:"pre_script"`
	// Parameter is for sonarQube type only
	Parameter string `bson:"parameter" json:"parameter"`
	// Script is for other type only
	Script           string                   `bson:"script"                json:"script"`
	AdvancedSetting  *ScanningAdvancedSetting `bson:"advanced_setting"      json:"advanced_setting"`
	CheckQualityGate bool                     `bson:"check_quality_gate"    json:"check_quality_gate"`
	Outputs          []*Output                `bson:"outputs"               json:"outputs"`
	NotifyCtls       []*NotifyCtl             `bson:"notify_ctls"     json:"notify_ctls"`

	CreatedAt int64  `bson:"created_at" json:"created_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
	UpdatedBy string `bson:"updated_by" json:"updated_by"`
}

func (Scanning) TableName() string {
	return "scanning"
}

type ScanningAdvancedSetting struct {
	ClusterID  string              `bson:"cluster_id"   json:"cluster_id"`
	Timeout    int64               `bson:"timeout"      json:"timeout"`
	ResReq     setting.Request     `bson:"res_req"      json:"res_req"`
	ResReqSpec setting.RequestSpec `bson:"res_req_spec" json:"res_req_spec"`
	HookCtl    *ScanningHookCtl    `bson:"hook_ctl"     json:"hook_ctl"`
	NotifyCtls []*NotifyCtl        `bson:"notify_ctls"     json:"notify_ctls"`
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
