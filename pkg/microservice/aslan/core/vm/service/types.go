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

package service

import (
	"fmt"
)

type CreateVMRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Provider         string   `json:"provider"`
	Tags             []string `json:"tags"`
	VMIP             string   `json:"ip"`
	VMPort           int      `json:"port"`
	VMUser           string   `json:"user"`
	LoginHost        bool     `json:"login_host"`
	SSHPrivateKey    string   `json:"ssh_private_key"`
	ScheduleWorkflow bool     `json:"schedule_workflow"`
	Workspace        string   `json:"workspace"`
	TaskConcurrency  int      `json:"task_concurrency"`
}

func (args *CreateVMRequest) Validate() error {
	if args.Name == "" {
		return fmt.Errorf("name is required")
	}
	if args.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	return nil
}

type AgentBriefListResp struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	IP           string `json:"ip"`
}

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
	HostName      string `json:"host_name"`
	AgentVersion  string `json:"agent_version"`
	WorkDir       string `json:"work_dir"`
}

type RegisterAgentResponse struct {
	Token            string `json:"token"`
	Description      string `json:"description"`
	VmName           string `json:"vm_name"`
	ScheduleWorkflow bool   `json:"schedule_workflow"`
	TaskConcurrency  int    `json:"task_concurrency"`
	AgentVersion     string `json:"agent_version"`
	ZadigVersion     string `json:"zadig_version"`
	WorkDir          string `json:"work_dir"`
}

type RegisterAgentRequest struct {
	Token      string                   `json:"token"`
	Parameters *RegisterAgentParameters `json:"parameters"`
}

func (args *RegisterAgentRequest) Validate() error {
	if args.Token == "" {
		return fmt.Errorf("token is required")
	}
	return nil
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

type HeartbeatParameters struct {
	MemTotal      uint64 `json:"mem_total"`
	UsedMem       uint64 `json:"used_mem"`
	DiskSpace     uint64 `json:"disk_space"`
	FreeDiskSpace uint64 `json:"free_disk_space"`
	VMname        string `json:"vm_name"`
}

type HeartbeatRequest struct {
	Token      string               `json:"token"`
	Parameters *HeartbeatParameters `json:"parameters"`
}

type HeartbeatResponse struct {
	NeedOffline            bool         `json:"need_offline"`
	NeedUpdateAgentVersion bool         `json:"need_update_agent_version"`
	AgentVersion           string       `json:"agent_version"`
	ScheduleWorkflow       bool         `json:"schedule_workflow"`
	WorkDir                string       `json:"work_dir"`
	Concurrency            int          `json:"concurrency"`
	CacheType              string       `json:"cache_type"`
	CachePath              string       `json:"cache_path"`
	Object                 ObjectConfig `json:"object"`
	ServerURL              string       `json:"server_url"`
	VmName                 string       `json:"vm_name"`
	Description            string       `json:"description"`
	ZadigVersion           string       `json:"zadig_version"`
}

type ObjectConfig struct {
}

type PollingJobArgs struct {
	Token string `json:"token"`
}

type DownloadFileArgs struct {
	Token string `json:"token"`
}

type ReportJobArgs struct {
	Seq       int    `json:"seq"`
	Token     string `json:"token"`
	JobID     string `json:"job_id"`
	JobStatus string `json:"job_status"`
	JobLog    string `json:"job_log"`
	JobError  string `json:"job_error"`
	JobOutput []byte `json:"job_output"`
}

type AgentAccessCmds struct {
	LinuxPlatform *AgentAccessCmd `json:"linux_platform,omitempty"`
	MacOSPlatform *AgentAccessCmd `json:"macos_platform,omitempty"`
	WinPlatform   *AgentAccessCmd `json:"windows_platform,omitempty"`
}

type AgentAccessCmd struct {
	AMD64 map[string]string `json:"amd64"`
	ARM64 map[string]string `json:"arm64"`
}

type AgentDetails struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Provider         string   `json:"provider"`
	Tags             []string `json:"tags"`
	VMIP             string   `json:"ip"`
	VMPort           int      `json:"port"`
	VMUser           string   `json:"user"`
	LoginHost        bool     `json:"login_host"`
	SSHPrivateKey    string   `json:"ssh_private_key"`
	ScheduleWorkflow bool     `json:"schedule_workflow"`
	Workspace        string   `json:"workspace"`
	TaskConcurrency  int      `json:"task_concurrency"`
	Status           string   `json:"status"`
	Error            string   `json:"error"`
	Platform         string   `json:"platform"`
	Architecture     string   `json:"architecture"`
}

type PollingJobResp struct {
	ID            string `json:"id"`
	ProjectName   string `json:"project_name"`
	WorkflowName  string `json:"workflow_name"`
	TaskID        int64  `json:"task_id"`
	JobOriginName string `json:"job_origin_name"`
	JobName       string `json:"job_name"`
	JobType       string `json:"job_type"`
	Status        string `json:"status"`
	JobCtx        string `json:"job_ctx"`
}
