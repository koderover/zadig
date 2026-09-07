package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TerminalCommand struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	SessionID    string             `bson:"session_id"       json:"session_id"`
	Seq          int64              `bson:"seq"              json:"seq"`
	Command      string             `bson:"command"          json:"command"`
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
