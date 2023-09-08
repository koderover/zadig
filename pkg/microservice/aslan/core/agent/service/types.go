package service

import "fmt"

type CreateAgentRequest struct {
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
	Hostname      string `json:"hostname"`
}

type RegisterAgentResponse struct {
	Token        string `json:"token"`
	Description  string `json:"description"`
	ExecutorNum  int    `json:"executor_num"`
	AgentVersion string `json:"agent_version"`
	ZadigVersion string `json:"zadig_version"`
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
	Hostname      string `json:"hostname"`
}

type HeartbeatRequest struct {
	Token      string `json:"token"`
	Parameters *HeartbeatParameters
}

type HeartbeatResponse struct {
	NeedUpdateAgentVersion bool   `json:"need_update_agent_version"`
	AgentVersion           string `json:"agent_version"`
	NeedUninstallAgent     bool   `json:"need_uninstall_agent"`
}
