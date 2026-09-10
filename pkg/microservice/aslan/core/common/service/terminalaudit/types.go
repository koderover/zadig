package terminalaudit

import (
	"io"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

const (
	defaultCols = 135
	defaultRows = 40
)

type SessionMeta struct {
	SessionType   models.TerminalSessionType
	Protocol      string
	UserID        string
	Username      string
	Account       string
	ProjectName   string
	EnvName       string
	ServiceName   string
	WorkflowName  string
	JobName       string
	TaskID        int64
	TargetName    string
	RemoteAddr    string
	LoginAccount  string
	HostID        string
	HostName      string
	HostIP        string
	ClusterID     string
	Namespace     string
	PodName       string
	ContainerName string
	ClientIP      string
	UserAgent     string
	InitialCols   int
	InitialRows   int
	// Secrets stores raw secret values to be masked from recordings.
	Secrets []string
}

// SessionListResponse keeps pagination metadata alongside the session collection.
type SessionListResponse struct {
	Total    int64                     `json:"total"`
	Sessions []*models.TerminalSession `json:"sessions"`
}

// CommandListResponse keeps pagination metadata alongside the command collection.
type CommandListResponse struct {
	Total    int64                     `json:"total"`
	Commands []*models.TerminalCommand `json:"commands"`
}

// CastFileStream couples the cast body with its stored size for HTTP streaming.
type CastFileStream struct {
	Body     io.ReadCloser
	FileSize int64
}
