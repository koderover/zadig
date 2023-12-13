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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PrivateKey struct {
	ID               primitive.ObjectID   `bson:"_id,omitempty"          json:"id,omitempty"`
	Name             string               `bson:"name"                   json:"name"`
	Description      string               `bson:"description"            json:"description"`
	UserName         string               `bson:"user_name"              json:"user_name"`
	IP               string               `bson:"ip"                     json:"ip"`
	Port             int64                `bson:"port"                   json:"port"`
	Status           setting.PMHostStatus `bson:"status"                 json:"status"`
	Label            string               `bson:"label"                  json:"label"`
	IsProd           bool                 `bson:"is_prod"                json:"is_prod"`
	IsLogin          bool                 `bson:"is_login"               json:"is_login"`
	PrivateKey       string               `bson:"private_key"            json:"private_key"`
	CreateTime       int64                `bson:"create_time"            json:"create_time"`
	UpdateTime       int64                `bson:"update_time"            json:"update_time"`
	UpdateBy         string               `bson:"update_by"              json:"update_by"`
	Provider         int8                 `bson:"provider"               json:"provider"`
	Probe            *types.Probe         `bson:"probe"                  json:"probe"`
	ProjectName      string               `bson:"project_name,omitempty" json:"project_name"`
	UpdateStatus     bool                 `bson:"-"                      json:"update_status"`
	ScheduleWorkflow bool                 `bson:"schedule_workflow"      json:"schedule_workflow"`
	Error            string               `bson:"error"                  json:"error"`
	Agent            *VMAgent             `bson:"agent"                  json:"agent,omitempty"`
	VMInfo           *VMInfo              `bson:"vm_info"                json:"vm_info,omitempty"`
	Type             string               `bson:"type"                   json:"type"`
}

type VMInfo struct {
	IP            string `bson:"ip"                   json:"ip"`
	Platform      string `bson:"platform"             json:"platform"`
	Architecture  string `bson:"architecture"         json:"architecture"`
	MemeryTotal   uint64 `bson:"memery_total"         json:"memery_total"`
	UsedMemery    uint64 `bson:"used_memery"          json:"used_memery"`
	CpuNum        int    `bson:"cpu_num"              json:"cpu_num"`
	DiskSpace     uint64 `bson:"disk_space"           json:"disk_space"`
	FreeDiskSpace uint64 `bson:"free_disk_space"      json:"free_disk_space"`
	HostName      string `bson:"host_name"            json:"host_name"`
}

type VMAgent struct {
	Token             string `bson:"token"                json:"-"`
	Workspace         string `bson:"workspace"            json:"workspace"`
	TaskConcurrency   int    `bson:"task_concurrency"     json:"task_concurrency"`
	CacheType         string `bson:"cache_type"           json:"cache_type"`
	CachePath         string `bson:"cache_path"           json:"cache_path"`
	ObjectID          string `bson:"object_id"            json:"object_id"`
	NeedUpdate        bool   `bson:"need_update"          json:"need_update"`
	AgentVersion      string `bson:"agent_version"        json:"agent_version"`
	ZadigVersion      string `bson:"zadig_version"        json:"zadig_version"`
	LastHeartbeatTime int64  `bson:"last_heartbeat_time"  json:"last_heartbeat_time"`
}

func (PrivateKey) TableName() string {
	return "private_key"
}

func (args *PrivateKey) Validate() error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
		licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
		licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		if args.Provider == config.VMProviderAmazon {
			return e.ErrLicenseInvalid.AddDesc("")
		}
	}
	return nil
}
