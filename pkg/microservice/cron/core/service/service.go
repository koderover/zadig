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

package service

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type SvcRevision struct {
	ServiceName     string       `json:"service_name"`
	Type            string       `json:"type"`
	CurrentRevision int64        `json:"current_revision"`
	NextRevision    int64        `json:"next_revision"`
	Updatable       bool         `json:"updatable"`
	Deleted         bool         `json:"deleted"`
	New             bool         `json:"new"`
	Containers      []*Container `json:"containers,omitempty"`
}

type ProductRevision struct {
	ID          string `json:"id,omitempty"`
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
	// 表示该产品更新前版本
	CurrentRevision int64 `json:"current_revision"`
	// 表示该产品更新后版本
	NextRevision int64 `json:"next_revision"`
	// ture: 表示该产品的服务发生变化, 需要更新
	// false: 表示该产品的服务未发生变化, 无需更新
	Updatable bool `json:"updatable"`
	// 可以自动更新产品, 展示用户更新前和更新后的服务组以及服务详细对比
	ServiceRevisions []*SvcRevision `json:"services"`
	IsPublic         bool           `json:"isPublic"`
}

type EnvResource struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	Type           string             `bson:"type"                      json:"type"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	DeletedAt      int64              `bson:"deleted_at"                json:"deleted_at" `
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
	AutoSync       bool               `bson:"auto_sync"                 json:"auto_sync"`
}

type ProductResp struct {
	ID          string      `json:"id"`
	ProductName string      `json:"product_name"`
	Namespace   string      `json:"namespace"`
	Status      string      `json:"status"`
	Error       string      `json:"error"`
	EnvName     string      `json:"env_name"`
	UpdateBy    string      `json:"update_by"`
	UpdateTime  int64       `json:"update_time"`
	Services    [][]string  `json:"services"`
	Render      *RenderInfo `json:"render"`
	Vars        []*RenderKV `json:"vars"`
	IsPublic    bool        `json:"isPublic"`
	ClusterID   string      `json:"cluster_id,omitempty"`
	RecycleDay  int         `json:"recycle_day"`
	IsProd      bool        `json:"is_prod"`
	Source      string      `json:"source"`
}

type ProductRenderset struct {
	Name        string                          `bson:"name"                     json:"name"`
	Revision    int64                           `bson:"revision"                 json:"revision"`
	EnvName     string                          `bson:"env_name,omitempty"       json:"env_name,omitempty"`
	ProductTmpl string                          `bson:"product_tmpl"             json:"product_tmpl"`
	YamlData    *templatemodels.CustomYaml      `bson:"yaml_data,omitempty"            json:"yaml_data,omitempty"`
	ChartInfos  []*templatemodels.ServiceRender `bson:"chart_infos,omitempty"    json:"chart_infos,omitempty"`
}

type EnvConfig struct {
	EnvName string   `json:"env_name"`
	HostIDs []string `json:"host_ids"`
	Labels  []string `json:"labels"`
}

type Service struct {
	ServiceName  string           `json:"service_name"`
	ProductName  string           `json:"product_name"`
	Revision     int64            `json:"revision"`
	HealthChecks []*PmHealthCheck `json:"health_checks,omitempty"`
	EnvConfigs   []*EnvConfig     `json:"env_configs,omitempty"`
	EnvStatuses  []*EnvStatus     `json:"env_statuses,omitempty"`
}

type PmHealthCheck struct {
	Protocol            string `json:"protocol,omitempty"`
	Port                int    `json:"port,omitempty"`
	Path                string `json:"path,omitempty"`
	TimeOut             int64  `json:"time_out,omitempty"`
	Interval            uint64 `json:"interval,omitempty"`
	HealthyThreshold    int    `json:"healthy_threshold,omitempty"`
	UnhealthyThreshold  int    `json:"unhealthy_threshold,omitempty"`
	CurrentHealthyNum   int    `json:"current_healthy_num,omitempty"`
	CurrentUnhealthyNum int    `json:"current_unhealthy_num,omitempty"`
}

type PrivateKeyHosts struct {
	ID           primitive.ObjectID   `json:"id,omitempty"`
	IP           string               `json:"ip"`
	Port         int64                `json:"port"`
	Status       setting.PMHostStatus `json:"status"`
	Probe        *types.Probe         `json:"probe"`
	UpdateStatus bool                 `json:"update_status"`
}

type EnvStatus struct {
	HostID        string         `json:"host_id"`
	EnvName       string         `json:"env_name"`
	Address       string         `json:"address"`
	Status        string         `json:"status"`
	PmHealthCheck *PmHealthCheck `json:"health_checks"`
}

// ServiceTmplObject ...
type ServiceTmplObject struct {
	ProductName string       `json:"product_name"`
	ServiceName string       `json:"service_name"`
	Revision    int64        `json:"revision"`
	Type        string       `json:"type"`
	Username    string       `json:"username"`
	EnvStatuses []*EnvStatus `json:"env_statuses,omitempty"`
}
