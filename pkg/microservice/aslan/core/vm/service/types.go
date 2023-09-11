package service

import "fmt"

type CreateHostRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Provider         string   `json:"provider"`
	Tags             []string `json:"tags"`
	HostIP           string   `json:"host_ip"`
	HostPort         int      `json:"host_port"`
	HostUser         string   `json:"host_user"`
	LoginHost        bool     `json:"login_host"`
	SSHPrivateKey    string   `json:"ssh_private_key"`
	ScheduleWorkflow bool     `json:"schedule_workflow"`
	Workspace        string   `json:"workspace"`
	TaskConcurrency  int      `json:"task_concurrency"`
}

func (args *CreateHostRequest) Validate() error {
	if args.Name == "" {
		return fmt.Errorf("name is required")
	}
	if args.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	return nil
}

type AgentBriefListResp struct {
	Name         string   `json:"name"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
	Error        string   `json:"error"`
	Platform     string   `json:"platform"`
	Architecture string   `json:"architecture"`
	IP           string   `json:"ip"`
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
	Token      string               `json:"token"`
	Parameters *HeartbeatParameters `json:"parameters"`
}

type HeartbeatResponse struct {
	NeedUpdateAgentVersion bool   `json:"need_update_agent_version"`
	AgentVersion           string `json:"agent_version"`
	NeedUninstallAgent     bool   `json:"need_uninstall_agent"`
}

type PollingJobArgs struct {
	Token string `json:"token"`
}

type ReportJobArgs struct {
	Token string `json:"token"`
}

type AgentAccessCmd struct {
	Architecture string `json:"architecture"`
	Cmd          string `json:"cmd"`
}

type AgentDetails struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Provider         string   `json:"provider"`
	Tags             []string `json:"tags"`
	HostIP           string   `json:"host_ip"`
	HostPort         int      `json:"host_port"`
	HostUser         string   `json:"host_user"`
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
