package terminalaudit

import (
	"strings"

	"github.com/google/shlex"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type AuditEvidenceCoverage string

const (
	AuditEvidenceCoverageComplete AuditEvidenceCoverage = "complete"
	AuditEvidenceCoveragePartial  AuditEvidenceCoverage = "partial"
)

// TerminalAuditEvidence contains the terminal data that can be reviewed without
// making assumptions about commands whose source files were not recorded.
type TerminalAuditEvidence struct {
	Session  TerminalAuditSessionEvidence   `json:"session"`
	Commands []TerminalAuditCommandEvidence `json:"commands"`
	Coverage AuditEvidenceCoverage          `json:"coverage"`
}

type TerminalAuditSessionEvidence struct {
	SessionID     string                       `json:"session_id"`
	SessionType   models.TerminalSessionType   `json:"session_type"`
	Status        models.TerminalSessionStatus `json:"status"`
	Username      string                       `json:"username"`
	Account       string                       `json:"account"`
	ProjectName   string                       `json:"project_name"`
	EnvName       string                       `json:"env_name"`
	ServiceName   string                       `json:"service_name"`
	WorkflowName  string                       `json:"workflow_name"`
	JobName       string                       `json:"job_name"`
	TargetName    string                       `json:"target_name"`
	Protocol      string                       `json:"protocol"`
	RemoteAddr    string                       `json:"remote_addr"`
	LoginAccount  string                       `json:"login_account"`
	HostName      string                       `json:"host_name"`
	HostIP        string                       `json:"host_ip"`
	Namespace     string                       `json:"namespace"`
	PodName       string                       `json:"pod_name"`
	ContainerName string                       `json:"container_name"`
}

type TerminalAuditCommandEvidence struct {
	Seq             int64  `json:"seq"`
	TimeOffsetMS    int64  `json:"time_offset_ms"`
	Command         string `json:"command"`
	OpaqueExecution string `json:"opaque_execution,omitempty"`
}

// BuildTerminalAuditEvidence builds the AI audit input from persisted session
// metadata and commands. Terminal recordings are reserved for playback.
func BuildTerminalAuditEvidence(session *models.TerminalSession, commands []*models.TerminalCommand) *TerminalAuditEvidence {
	evidence := &TerminalAuditEvidence{
		Session: TerminalAuditSessionEvidence{
			SessionID:     session.SessionID,
			SessionType:   session.SessionType,
			Status:        session.Status,
			Username:      session.Username,
			Account:       session.Account,
			ProjectName:   session.ProjectName,
			EnvName:       session.EnvName,
			ServiceName:   session.ServiceName,
			WorkflowName:  session.WorkflowName,
			JobName:       session.JobName,
			TargetName:    session.TargetName,
			Protocol:      session.Protocol,
			RemoteAddr:    session.RemoteAddr,
			LoginAccount:  session.LoginAccount,
			HostName:      session.HostName,
			HostIP:        session.HostIP,
			Namespace:     session.Namespace,
			PodName:       session.PodName,
			ContainerName: session.ContainerName,
		},
		Commands: make([]TerminalAuditCommandEvidence, 0, len(commands)),
		Coverage: AuditEvidenceCoverageComplete,
	}
	for _, command := range commands {
		commandEvidence := TerminalAuditCommandEvidence{
			Seq:          command.Seq,
			TimeOffsetMS: command.TimeOffsetMS,
			Command:      command.Command,
		}
		if reason, ok := detectOpaqueExecution(command.Command); ok {
			commandEvidence.OpaqueExecution = reason
			evidence.Coverage = AuditEvidenceCoveragePartial
		}
		evidence.Commands = append(evidence.Commands, commandEvidence)
	}
	return evidence
}

func detectOpaqueExecution(command string) (string, bool) {
	segments := strings.Split(command, "|")
	for _, segment := range segments[1:] {
		fields, err := shlex.Split(segment)
		if err != nil {
			continue
		}
		executable, _ := unwrapCommandPrefixes(fields)
		if isShellInterpreter(executable) {
			return "remote_script_content_unavailable", true
		}
	}

	fields, err := shlex.Split(segments[0])
	if err != nil {
		return "", false
	}
	executable, args := unwrapCommandPrefixes(fields)
	if executable == "" {
		return "", false
	}
	if executable == "source" || executable == "." {
		if len(args) > 0 {
			return "script_content_unavailable", true
		}
		return "", false
	}
	if isShellInterpreter(executable) {
		for i, field := range args {
			if field == "-c" || field == "-e" {
				if i+1 < len(args) && strings.HasPrefix(args[i+1], "$") {
					return "script_content_unavailable", true
				}
				return "", false
			}
			if strings.HasPrefix(field, "-") {
				continue
			}
			if isScriptPath(field) {
				return "script_content_unavailable", true
			}
		}
	}
	if isScriptPath(executable) {
		return "script_content_unavailable", true
	}
	return "", false
}

// unwrapCommandPrefixes returns the actual executable and its arguments after
// removing leading environment assignments and env/sudo options.
func unwrapCommandPrefixes(fields []string) (string, []string) {
	prefixOptions := false
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if field == "env" || field == "sudo" {
			prefixOptions = true
			continue
		}
		if isEnvironmentAssignment(field) {
			continue
		}
		if prefixOptions && strings.HasPrefix(field, "-") {
			switch field {
			case "-u", "-g", "-h", "-p", "-C", "-T", "--user", "--group",
				"--host", "--prompt", "--close-from", "--command-timeout":
				i++
			}
			continue
		}
		return field, fields[i+1:]
	}
	return "", nil
}

func isEnvironmentAssignment(field string) bool {
	name, _, ok := strings.Cut(field, "=")
	if !ok || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}

func isShellInterpreter(value string) bool {
	value = strings.TrimSuffix(value, "\r")
	parts := strings.Split(value, "/")
	switch parts[len(parts)-1] {
	case "sh", "bash", "dash", "zsh", "ksh", "fish", "python", "python3", "perl", "ruby", "node":
		return true
	default:
		return false
	}
}

func isScriptPath(value string) bool {
	value = strings.Trim(value, "'\"")
	for _, suffix := range []string{".sh", ".bash", ".zsh", ".py", ".pl", ".rb", ".js"} {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
