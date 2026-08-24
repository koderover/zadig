package terminalaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

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
	Session      TerminalAuditSessionEvidence   `json:"session"`
	Commands     []TerminalAuditCommandEvidence `json:"commands"`
	Unattributed []TerminalAuditEvent           `json:"unattributed"`
	Coverage     AuditEvidenceCoverage          `json:"coverage"`
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
	Output          string `json:"nearby_output"`
	OpaqueExecution string `json:"opaque_execution,omitempty"`
}

type TerminalAuditEvent struct {
	OffsetMS int64  `json:"offset_ms"`
	Type     string `json:"type"`
	Data     string `json:"data"`
}

// BuildTerminalAuditEvidence builds a deterministic snapshot while retaining
// at most maxDataRunes runes of cast event data. A non-negative endOffsetMS
// stops parsing before the first command excluded by the caller.
func BuildTerminalAuditEvidence(session *models.TerminalSession, commands []*models.TerminalCommand, cast io.Reader, maxDataRunes int, endOffsetMS int64) (*TerminalAuditEvidence, error) {
	if maxDataRunes <= 0 {
		return nil, errors.New("terminal audit evidence data limit must be positive")
	}
	if session == nil {
		return nil, errors.New("terminal session is nil")
	}
	if cast == nil {
		return nil, errors.New("terminal cast reader is nil")
	}

	sortedCommands := make([]*models.TerminalCommand, 0, len(commands))
	for _, command := range commands {
		if command == nil {
			return nil, errors.New("terminal command is nil")
		}
		sortedCommands = append(sortedCommands, command)
	}
	sort.SliceStable(sortedCommands, func(i, j int) bool {
		if sortedCommands[i].TimeOffsetMS == sortedCommands[j].TimeOffsetMS {
			return sortedCommands[i].Seq < sortedCommands[j].Seq
		}
		return sortedCommands[i].TimeOffsetMS < sortedCommands[j].TimeOffsetMS
	})

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
		Commands: make([]TerminalAuditCommandEvidence, 0, len(sortedCommands)),
		Coverage: AuditEvidenceCoverageComplete,
	}
	for _, command := range sortedCommands {
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

	decoder := json.NewDecoder(cast)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode asciicast header: %w", err)
	}
	var header castHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, fmt.Errorf("decode asciicast header: %w", err)
	}
	if header.Version != 2 {
		return nil, fmt.Errorf("unsupported asciicast version %d", header.Version)
	}

	outputBuilders := make([]strings.Builder, len(evidence.Commands))
	retainedDataRunes := 0
	for {
		if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode asciicast event: %w", err)
		}

		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil, fmt.Errorf("decode asciicast event: %w", err)
		}
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid asciicast event: expected 3 fields, got %d", len(parts))
		}

		var typ string
		if err := json.Unmarshal(parts[1], &typ); err != nil {
			return nil, fmt.Errorf("decode asciicast event type: %w", err)
		}
		if typ != "i" && typ != "o" && typ != "r" {
			continue
		}
		var offset float64
		if err := json.Unmarshal(parts[0], &offset); err != nil {
			return nil, fmt.Errorf("decode asciicast event offset: %w", err)
		}
		offsetMS := int64(offset*1000 + 0.5)
		if endOffsetMS >= 0 && offsetMS >= endOffsetMS {
			evidence.Coverage = AuditEvidenceCoveragePartial
			break
		}
		var data string
		if err := json.Unmarshal(parts[2], &data); err != nil {
			return nil, fmt.Errorf("decode asciicast event data: %w", err)
		}

		// The budget is shared by attributed output and unattributed events. Once
		// exhausted, stop decoding the stream so memory usage no longer follows the
		// total cast file size.
		remainingRunes := maxDataRunes - retainedDataRunes
		if remainingRunes == 0 {
			evidence.Coverage = AuditEvidenceCoveragePartial
			break
		}

		eventRunes := utf8.RuneCountInString(data)
		truncated := eventRunes > remainingRunes
		if eventRunes > remainingRunes {
			data = string([]rune(data)[:remainingRunes])
			eventRunes = remainingRunes
		}
		retainedDataRunes += eventRunes

		commandIndex := sort.Search(len(evidence.Commands), func(i int) bool {
			return evidence.Commands[i].TimeOffsetMS > offsetMS
		}) - 1
		if typ == "o" && commandIndex >= 0 {
			outputBuilders[commandIndex].WriteString(data)
		} else {
			evidence.Unattributed = append(evidence.Unattributed, TerminalAuditEvent{
				OffsetMS: offsetMS,
				Type:     typ,
				Data:     data,
			})
		}
		if truncated {
			evidence.Coverage = AuditEvidenceCoveragePartial
			break
		}
	}
	for i := range evidence.Commands {
		evidence.Commands[i].Output = outputBuilders[i].String()
	}
	return evidence, nil
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
