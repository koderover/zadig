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
	"fmt"

	internalresource "github.com/koderover/zadig/lib/internal/kube/resource"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
)

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

type SvcRevision struct {
	ServiceName     string                    `json:"service_name"`
	Type            string                    `json:"type"`
	CurrentRevision int64                     `json:"current_revision"`
	NextRevision    int64                     `json:"next_revision"`
	Updatable       bool                      `json:"updatable"`
	Deleted         bool                      `json:"deleted"`
	New             bool                      `json:"new"`
	ConfigRevisions []*ConfigRevision         `json:"configs,omitempty"`
	Containers      []*commonmodels.Container `json:"containers,omitempty"`
}

type ConfigRevision struct {
	ConfigName      string `json:"config_name"`
	CurrentRevision int64  `json:"current_revision"`
	NextRevision    int64  `json:"next_revision"`
	Updatable       bool   `json:"updatable"`
	Deleted         bool   `json:"deleted"`
	New             bool   `json:"new"`
}

type ProductIngressInfo struct {
	IngressInfos []*commonservice.IngressInfo `json:"ingress_infos"`
	EnvName      string                       `json:"env_name"`
}

type SvcOptArgs struct {
	EnvName     string
	ProductName string
	ServiceName string
	ServiceType string
	ServiceRev  *SvcRevision
	UpdateBy    string
}

type RestartScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
}

type ScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
	Number      int    `json:"number"`
}

// SvcResp struct 产品-服务详情页面Response
type SvcResp struct {
	ServiceName string                       `json:"service_name"`
	Scales      []*internalresource.Workload `json:"scales"`
	Ingress     []*internalresource.Ingress  `json:"ingress"`
	Services    []*internalresource.Service  `json:"service_endpoints"`
	EnvName     string                       `json:"env_name"`
	ProductName string                       `json:"product_name"`
	GroupName   string                       `json:"group_name"`
}

func (pr *ProductRevision) GroupsUpdated() bool {
	if pr.ServiceRevisions == nil || len(pr.ServiceRevisions) == 0 {
		return false
	}
	for _, serviceRev := range pr.ServiceRevisions {
		if serviceRev.Updatable {
			return true
		}
	}
	return pr.Updatable
}

func (sr *SvcRevision) ConfigsUpdated() bool {
	if sr.ConfigRevisions == nil || len(sr.ConfigRevisions) == 0 {
		return false
	}
	for _, configRev := range sr.ConfigRevisions {
		if configRev.Updatable {
			return true
		}
	}
	return sr.Updatable
}

type ContainerNotFound struct {
	ServiceName string
	Container   string
	EnvName     string
	ProductName string
}

func (c *ContainerNotFound) Error() string {
	return fmt.Sprintf("serviceName:%s,container:%s", c.ServiceName, c.Container)
}
