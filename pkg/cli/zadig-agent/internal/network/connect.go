/*
Copyright 2023 The KodeRover Authors.

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

package network

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	httpclient "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/client"
)

const (
	RegisterBaseUrl     = "/api/aslan/vm/agents/register"
	VerifyBaseUrl       = "/api/aslan/vm/agents/verify"
	heartbeatBaseUrl    = "/api/aslan/vm/agents/heartbeat"
	RequestJobBaseUrl   = "/api/aslan/vm/agents/job/request"
	ReportJobBaseUrl    = "/api/aslan/vm/agents/job/report"
	DownloadFileBaseUrl = "/api/aslan/vm/agents/tempFile/download/%s"
)

type RegisterAgentParameters struct {
	IP            string `json:"ip"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CurrentUser   string `json:"current_user"`
	MemTotal      uint64 `json:"mem_total"`
	UsedMem       uint64 `json:"used_mem"`
	CpuNum        int    `json:"cpu_num"`
	DiskSpace     uint64 `json:"disk_space"`
	FreeDiskSpace uint64 `json:"free_disk_space"`
	Hostname      string `json:"hostname"`
	AgentVersion  string `json:"agent_version"`
	WorkDir       string `json:"work_dir"`
}

type RegisterAgentResponse struct {
	AgentID          string `json:"agent_id"`
	VmName           string `json:"vm_name"`
	Token            string `json:"token"`
	Description      string `json:"description"`
	Concurrency      int    `json:"task_concurrency"`
	AgentVersion     string `json:"agent_version"`
	ZadigVersion     string `json:"zadig_version"`
	ScheduleWorkflow bool   `json:"schedule_workflow"`
	WorkDir          string `json:"work_dir"`
}

type RegisterAgentRequest struct {
	Token      string                   `json:"token"`
	Parameters *RegisterAgentParameters `json:"parameters"`
}

func RegisterAgent(config *AgentConfig, parameters *RegisterAgentParameters) (*RegisterAgentResponse, error) {
	request := &RegisterAgentRequest{
		Token:      config.Token,
		Parameters: parameters,
	}
	resp := new(RegisterAgentResponse)
	retry := 3

	var errMsg string
	for retry > 0 {
		body, err := httpclient.Post(GetFullURL(config.URL, RegisterBaseUrl), httpclient.SetBody(request), httpclient.SetResult(resp))
		if err != nil {
			return nil, err
		}

		if body.IsSuccess() {
			break
		}

		if body.IsError() {
			errMsg = httpclient.NewErrorFromRestyResponse(body).Error()
			log.Errorf("failed to register agent, error: %s", errMsg)

		}
		retry--
	}

	if retry == 0 {
		return nil, fmt.Errorf("failed to register agent, error: %s", errMsg)
	}
	return resp, nil
}

type VerifyAgentParameters struct {
	IP   string `json:"ip"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type VerifyAgentRequest struct {
	Token      string                 `json:"token"`
	Parameters *VerifyAgentParameters `json:"parameters"`
}

type VerifyAgentResponse struct {
	Verified bool `json:"verified"`
}

func VerifyAgent(config *AgentConfig, parameters *VerifyAgentParameters) (*VerifyAgentResponse, error) {
	request := &VerifyAgentRequest{
		Token:      config.Token,
		Parameters: parameters,
	}
	resp := new(VerifyAgentResponse)

	body, err := httpclient.Post(GetFullURL(config.URL, VerifyBaseUrl), httpclient.SetBody(request), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}

	if body.IsError() {
		errMsg := httpclient.NewErrorFromRestyResponse(body).Error()
		log.Errorf("failed to verify agent, error: %s", errMsg)
		return nil, fmt.Errorf("failed to verify agent, error: %s", errMsg)
	}

	if body.IsSuccess() {
		return resp, nil
	}
	return nil, fmt.Errorf("failed to verify agent")
}

type HeartbeatParameters struct {
	MemTotal      uint64 `json:"mem_total"`
	UsedMem       uint64 `json:"used_mem"`
	DiskSpace     uint64 `json:"disk_space"`
	FreeDiskSpace uint64 `json:"free_disk_space"`
	Hostname      string `json:"hostname"`
}

type HeartbeatServerRequest struct {
	Token      string               `json:"token"`
	Parameters *HeartbeatParameters `json:"parameters"`
}

type HeartbeatServerResponse struct {
	NeedOffline            bool   `json:"need_offline"`
	NeedUpdateAgentVersion bool   `json:"need_update_agent_version"`
	AgentVersion           string `json:"agent_version"`
	ScheduleWorkflow       bool   `json:"schedule_workflow"`
	WorkDir                string `json:"work_dir"`
	Concurrency            int    `json:"concurrency"`
	CacheType              string `json:"cache_type"`
	ServerURL              string `json:"server_url"`
	VmName                 string `json:"vm_name"`
	Description            string `json:"description"`
	ZadigVersion           string `json:"zadig_version"`
}

func Heartbeat(config *AgentConfig, parameters *HeartbeatParameters) (*HeartbeatServerResponse, error) {
	request := &HeartbeatServerRequest{
		Token:      config.Token,
		Parameters: parameters,
	}
	resp := new(HeartbeatServerResponse)
	body, err := httpclient.Post(GetFullURL(config.URL, heartbeatBaseUrl), httpclient.SetBody(request), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}

	if body.IsError() {
		errMsg := httpclient.NewErrorFromRestyResponse(body).Error()
		log.Errorf("failed to heartbeat zadig server, error: %s", errMsg)
		return nil, fmt.Errorf("failed to heartbeat zadig server, error: %s", errMsg)
	}

	if body.IsSuccess() {
		return resp, nil
	}
	return nil, fmt.Errorf("failed to heartbeat agent")
}
