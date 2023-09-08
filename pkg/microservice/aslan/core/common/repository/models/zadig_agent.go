package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type ZadigAgent struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"        json:"id,omitempty"`
	Token            string             `bson:"token"                json:"token"`
	Name             string             `bson:"name"                 json:"name"`
	Description      string             `bson:"description"          json:"description"`
	Provider         string             `bson:"provider"             json:"provider"`
	Tags             []string           `bson:"tags"                 json:"tags"`
	HostIP           string             `bson:"host_ip"              json:"host_ip"`
	HostPort         string             `bson:"host_port"            json:"host_port"`
	HostUser         string             `bson:"host_user"            json:"host_user"`
	IsProduction     bool               `bson:"is_production"        json:"is_production"`
	SSHPrivateKey    string             `bson:"ssh_private_key"      json:"ssh_private_key"`
	ScheduleWorkflow bool               `bson:"schedule_workflow"    json:"schedule_workflow"`
	WorkSpacePath    string             `bson:"work_space_path"      json:"work_space_path"`
	TaskConcurrency  int                `bson:"task_concurrency"     json:"task_concurrency"`
	NeedUpdate       bool               `bson:"need_update"          json:"need_update"`
	AgentVersion     string             `bson:"agent_version"        json:"agent_version"`
	ZadigVersion     string             `bson:"zadig_version"        json:"zadig_version"`
	IsDeleted        bool               `bson:"is_deleted"           json:"is_deleted"`
	Status           string             `bson:"status"               json:"status"`
	CreateTime       int64              `bson:"create_time"          json:"create_time"`
	CreatedBy        string             `bson:"created_by"           json:"created_by"`
	UpdateTime       int64              `bson:"update_time"          json:"update_time"`
	UpdateBy         string             `bson:"update_by"            json:"update_by"`
	HostInfo         *AgentHostInfo     `bson:"host_info"            json:"host_info"`
}

type AgentHostInfo struct {
	Platform      string `bson:"platform"             json:"platform"`
	Architecture  string `bson:"architecture"         json:"architecture"`
	MemeryTotal   uint64 `bson:"memery_total"         json:"memery_total"`
	UsedMemery    uint64 `bson:"used_memery"          json:"used_memery"`
	CpuNum        int    `bson:"cpu_num"              json:"cpu_num"`
	DiskSpace     uint64 `bson:"disk_space"           json:"disk_space"`
	FreeDiskSpace uint64 `bson:"free_disk_space"      json:"free_disk_space"`
	Hostname      string `bson:"hostname"             json:"hostname"`
}

func (ZadigAgent) TableName() string {
	return "zadig_agent"
}
