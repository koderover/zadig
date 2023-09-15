package vm

import "go.mongodb.org/mongo-driver/bson/primitive"

type ZadigVM struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"        json:"id,omitempty"`
	Token             string             `bson:"token"                json:"-"`
	Name              string             `bson:"name"                 json:"name"`
	Description       string             `bson:"description"          json:"description"`
	Provider          string             `bson:"provider"             json:"provider"`
	Tags              []string           `bson:"tags"                 json:"tags"`
	IP                string             `bson:"ip"                   json:"ip"`
	Port              int                `bson:"port"                 json:"port"`
	User              string             `bson:"user"                 json:"user"`
	SSHPrivateKey     string             `bson:"ssh_private_key"      json:"ssh_private_key"`
	ScheduleWorkflow  bool               `bson:"schedule_workflow"    json:"schedule_workflow"`
	Workspace         string             `bson:"workspace"            json:"workspace"`
	TaskConcurrency   int                `bson:"task_concurrency"     json:"task_concurrency"`
	NeedUpdate        bool               `bson:"need_update"          json:"need_update"`
	AgentVersion      string             `bson:"agent_version"        json:"agent_version"`
	ZadigVersion      string             `bson:"zadig_version"        json:"zadig_version"`
	IsDeleted         bool               `bson:"is_deleted"           json:"is_deleted"`
	Status            string             `bson:"status"               json:"status"`
	Error             string             `bson:"error"                json:"error"`
	CreateTime        int64              `bson:"create_time"          json:"create_time"`
	CreatedBy         string             `bson:"created_by"           json:"created_by"`
	UpdateTime        int64              `bson:"update_time"          json:"update_time"`
	UpdateBy          string             `bson:"update_by"            json:"update_by"`
	VMInfo            *VMInfo            `bson:"vm_info"              json:"vm_info"`
	LastHeartbeatTime int64              `bson:"last_heartbeat_time"  json:"last_heartbeat_time"`
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
	VMname        string `bson:"vm_name"              json:"vm_name"`
}

func (ZadigVM) TableName() string {
	return "zadig_host"
}
