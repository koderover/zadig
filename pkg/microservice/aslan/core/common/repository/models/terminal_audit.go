package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TerminalSessionType string

const (
	TerminalSessionTypeSSH           TerminalSessionType = "ssh"
	TerminalSessionTypePodExec       TerminalSessionType = "podexec"
	TerminalSessionTypeWorkflowDebug TerminalSessionType = "workflow_debug"
)

type TerminalSessionStatus string

const (
	TerminalSessionStatusRunning  TerminalSessionStatus = "running"
	TerminalSessionStatusFinished TerminalSessionStatus = "finished"
	TerminalSessionStatusAborted  TerminalSessionStatus = "aborted"
	TerminalSessionStatusFailed   TerminalSessionStatus = "failed"
)

type TerminalSession struct {
	ID              primitive.ObjectID    `bson:"_id,omitempty"       json:"id,omitempty"`
	SessionID       string                `bson:"session_id"          json:"session_id"`
	SessionType     TerminalSessionType   `bson:"session_type"        json:"session_type"`
	Status          TerminalSessionStatus `bson:"status"             json:"status"`
	UserID          string                `bson:"user_id"             json:"user_id"`
	Username        string                `bson:"username"            json:"username"`
	Account         string                `bson:"account"             json:"account"`
	ProjectName     string                `bson:"project_name"        json:"project_name"`
	EnvName         string                `bson:"env_name"            json:"env_name"`
	ServiceName     string                `bson:"service_name"        json:"service_name"`
	WorkflowName    string                `bson:"workflow_name"       json:"workflow_name"`
	JobName         string                `bson:"job_name"            json:"job_name"`
	TaskID          int64                 `bson:"task_id"             json:"task_id"`
	TargetName      string                `bson:"target_name"         json:"target_name"`
	Protocol        string                `bson:"protocol"            json:"protocol"`
	RemoteAddr      string                `bson:"remote_addr"         json:"remote_addr"`
	LoginAccount    string                `bson:"login_account"       json:"login_account"`
	HostID          string                `bson:"host_id"             json:"host_id"`
	HostName        string                `bson:"host_name"           json:"host_name"`
	HostIP          string                `bson:"host_ip"             json:"host_ip"`
	ClusterID       string                `bson:"cluster_id"          json:"cluster_id"`
	Namespace       string                `bson:"namespace"           json:"namespace"`
	PodName         string                `bson:"pod_name"            json:"pod_name"`
	ContainerName   string                `bson:"container_name"      json:"container_name"`
	ClientIP        string                `bson:"client_ip"           json:"client_ip"`
	UserAgent       string                `bson:"user_agent"          json:"user_agent"`
	StartedAt       int64                 `bson:"started_at"          json:"started_at"`
	EndedAt         int64                 `bson:"ended_at"            json:"ended_at"`
	DurationSeconds int64                 `bson:"duration_seconds"    json:"duration_seconds"`
	LastActivityAt  int64                 `bson:"last_activity_at"    json:"last_activity_at"`
	CommandCount    int64                 `bson:"command_count"       json:"command_count"`
	StorageID       string                `bson:"storage_id"          json:"storage_id"`
	Bucket          string                `bson:"bucket"              json:"bucket"`
	ObjectKey       string                `bson:"object_key"          json:"object_key"`
	FileSize        int64                 `bson:"file_size"           json:"file_size"`
	ErrorMessage    string                `bson:"error_message"       json:"error_message"`
	CreatedAt       int64                 `bson:"created_at"          json:"created_at"`
	UpdatedAt       int64                 `bson:"updated_at"          json:"updated_at"`
}

func (TerminalSession) TableName() string {
	return "terminal_session"
}

type TerminalCommand struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	SessionID    string             `bson:"session_id"       json:"session_id"`
	Seq          int64              `bson:"seq"              json:"seq"`
	Command      string             `bson:"command"          json:"command"`
	RiskLevel    string             `bson:"risk_level"       json:"risk_level"`
	UserID       string             `bson:"user_id"          json:"user_id"`
	Username     string             `bson:"username"         json:"username"`
	Account      string             `bson:"account"          json:"account"`
	ProjectName  string             `bson:"project_name"     json:"project_name"`
	EnvName      string             `bson:"env_name"         json:"env_name"`
	TargetName   string             `bson:"target_name"      json:"target_name"`
	Protocol     string             `bson:"protocol"         json:"protocol"`
	RemoteAddr   string             `bson:"remote_addr"      json:"remote_addr"`
	LoginAccount string             `bson:"login_account"    json:"login_account"`
	TimeOffsetMS int64              `bson:"time_offset_ms"   json:"time_offset_ms"`
	CreatedAt    int64              `bson:"created_at"       json:"created_at"`
}

func (TerminalCommand) TableName() string {
	return "terminal_command"
}

type TerminalSessionListArgs struct {
	Status      string `form:"status" json:"status"`
	SessionType string `form:"sessionType" json:"sessionType"`
	ProjectName string `form:"projectName" json:"projectName"`
	EnvName     string `form:"envName" json:"envName"`
	ServiceName string `form:"serviceName" json:"serviceName"`
	Username    string `form:"username" json:"username"`
	TargetName  string `form:"targetName" json:"targetName"`
	RemoteAddr  string `form:"remoteAddr" json:"remoteAddr"`
	StartTime   int64  `form:"startTime" json:"startTime"`
	EndTime     int64  `form:"endTime" json:"endTime"`
	PageNum     int64  `form:"pageNum" json:"pageNum"`
	PageSize    int64  `form:"pageSize" json:"pageSize"`
}

type TerminalCommandListArgs struct {
	SessionID   string `form:"sessionID" json:"sessionID"`
	ProjectName string `form:"projectName" json:"projectName"`
	Username    string `form:"username" json:"username"`
	TargetName  string `form:"targetName" json:"targetName"`
	RemoteAddr  string `form:"remoteAddr" json:"remoteAddr"`
	Command     string `form:"command" json:"command"`
	StartTime   int64  `form:"startTime" json:"startTime"`
	EndTime     int64  `form:"endTime" json:"endTime"`
	PageNum     int64  `form:"pageNum" json:"pageNum"`
	PageSize    int64  `form:"pageSize" json:"pageSize"`
}
