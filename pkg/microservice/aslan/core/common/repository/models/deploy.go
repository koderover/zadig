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

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type Deploy struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty"                json:"id,omitempty"`
	Name           string              `bson:"name"                         json:"name"`
	ServiceName    string              `bson:"service_name"                 json:"service_name"`
	ProjectName    string              `bson:"project_name"                 json:"project_name"`
	Source         string              `bson:"source,omitempty"             json:"source,omitempty"`
	Timeout        int                 `bson:"timeout"                      json:"timeout"`
	Description    string              `bson:"desc,omitempty"               json:"desc"`
	UpdateTime     int64               `bson:"update_time"                  json:"update_time"`
	UpdateBy       string              `bson:"update_by"                    json:"update_by"`
	Repos          []*types.Repository `bson:"repos"                        json:"repos"`
	Infrastructure string              `bson:"infrastructure"               json:"infrastructure"`
	VMLabels       []string            `bson:"vm_labels"                    json:"vm_labels"`
	ScriptType     types.ScriptType    `bson:"script_type"                  json:"script_type"`
	Scripts        string              `bson:"scripts"                      json:"scripts"`

	SSHs                     []string                   `bson:"sshs"                          json:"sshs"`
	PreDeploy                *PreDeploy                 `bson:"pre_deploy"                    json:"pre_deploy"`
	Type                     types.VMDeployType         `bson:"type"                          json:"type"`
	ArtifactType             types.VMDeployArtifactType `bson:"artifact_type"                 json:"artifact_type"`
	AdvancedSettingsModified bool                       `bson:"advanced_setting_modified"     json:"advanced_setting_modified"`

	Outputs []*Output `bson:"outputs"                   json:"outputs"`
}

type PreDeploy struct {
	ResReq              setting.Request     `bson:"res_req"                  json:"res_req"`
	ResReqSpec          setting.RequestSpec `bson:"res_req_spec"             json:"res_req_spec"`
	BuildOS             string              `bson:"build_os"                 json:"build_os"`
	ImageFrom           string              `bson:"image_from"               json:"image_from"`
	ImageID             string              `bson:"image_id"                 json:"image_id"`
	ClusterID           string              `bson:"cluster_id"               json:"cluster_id"`
	StrategyID          string              `bson:"strategy_id"              json:"strategy_id"`
	UseHostDockerDaemon bool                `bson:"use_host_docker_daemon"   json:"use_host_docker_daemon"`
	Installs            []*Item             `bson:"installs,omitempty"       json:"installs"`
	Envs                KeyValList          `bson:"envs,omitempty"           json:"envs"`
}

func (deploy *Deploy) SafeRepos() []*types.Repository {
	if len(deploy.Repos) == 0 {
		return []*types.Repository{}
	}
	return deploy.Repos
}

func (deploy *Deploy) SafeReposDeepCopy() []*types.Repository {
	if len(deploy.Repos) == 0 {
		return []*types.Repository{}
	}
	resp := make([]*types.Repository, 0)
	for _, repo := range deploy.Repos {
		tmpRepo := *repo
		resp = append(resp, &tmpRepo)
	}
	return resp
}

func (Deploy) TableName() string {
	return "module_deploy"
}
