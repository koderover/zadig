package terminalaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

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
	Session          TerminalAuditSessionEvidence   `json:"session"`
	Commands         []TerminalAuditCommandEvidence `json:"commands"`
	Unattributed     []TerminalAuditEvent           `json:"unattributed"`
	OpaqueExecutions []TerminalOpaqueExecution      `json:"opaque_executions"`
	Coverage         AuditEvidenceCoverage          `json:"coverage"`
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
	Seq               int64  `json:"seq"`
	TimeOffsetMS      int64  `json:"time_offset_ms"`
	Command           string `json:"command"`
	Output            string `json:"nearby_output"`
	OutputAttribution string `json:"output_attribution"`
}

type TerminalAuditEvent struct {
	OffsetMS int64  `json:"offset_ms"`
	Type     string `json:"type"`
	Data     string `json:"data"`
}

type TerminalOpaqueExecution struct {
	Seq     int64  `json:"seq"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type terminalAuditCastEvent struct {
	offsetMS int64
	typ      string
	data     string
}

// BuildTerminalAuditEvidence builds a deterministic snapshot while retaining
// at most maxDataRunes runes of cast event data. Reaching the limit stops cast
// parsing and marks the evidence partial.
func BuildTerminalAuditEvidence(session *models.TerminalSession, commands []*models.TerminalCommand, cast io.Reader, maxDataRunes int) (*TerminalAuditEvidence, error) {
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
		evidence.Commands = append(evidence.Commands, TerminalAuditCommandEvidence{
			Seq:               command.Seq,
			TimeOffsetMS:      command.TimeOffsetMS,
			Command:           command.Command,
			OutputAttribution: "time_window",
		})
		if reason, ok := detectOpaqueExecution(command.Command); ok {
			evidence.OpaqueExecutions = append(evidence.OpaqueExecutions, TerminalOpaqueExecution{
				Seq:     command.Seq,
				Command: command.Command,
				Reason:  reason,
			})
		}
	}
	if len(evidence.OpaqueExecutions) > 0 {
		evidence.Coverage = AuditEvidenceCoveragePartial
	}

	outputBuilders := make([]strings.Builder, len(evidence.Commands))
	retainedDataRunes := 0
	dataTruncated := false
	err := forEachTerminalAuditCastEvent(cast, func(event terminalAuditCastEvent) (bool, error) {
		// The budget is shared by attributed output and unattributed events. Once
		// exhausted, stop decoding the stream so memory usage no longer follows the
		// total cast file size.
		remainingRunes := maxDataRunes - retainedDataRunes
		if remainingRunes == 0 {
			dataTruncated = true
			return true, nil
		}

		eventRunes := utf8.RuneCountInString(event.data)
		if eventRunes > remainingRunes {
			event.data = string([]rune(event.data)[:remainingRunes])
			eventRunes = remainingRunes
			dataTruncated = true
		}
		retainedDataRunes += eventRunes

		commandIndex := commandIndexAt(evidence.Commands, event.offsetMS)
		if event.typ == "o" && commandIndex >= 0 {
			outputBuilders[commandIndex].WriteString(event.data)
			return dataTruncated, nil
		}
		evidence.Unattributed = append(evidence.Unattributed, TerminalAuditEvent{
			OffsetMS: event.offsetMS,
			Type:     event.typ,
			Data:     event.data,
		})
		return dataTruncated, nil
	})
	if err != nil {
		return nil, err
	}
	for i := range evidence.Commands {
		evidence.Commands[i].Output = outputBuilders[i].String()
	}
	if dataTruncated {
		evidence.Coverage = AuditEvidenceCoveragePartial
	}
	return evidence, nil
}

func forEachTerminalAuditCastEvent(reader io.Reader, fn func(terminalAuditCastEvent) (stop bool, err error)) error {
	decoder := json.NewDecoder(reader)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("decode asciicast header: %w", err)
	}
	var header castHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return fmt.Errorf("decode asciicast header: %w", err)
	}
	if header.Version != 2 {
		return fmt.Errorf("unsupported asciicast version %d", header.Version)
	}

	for {
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode asciicast event: %w", err)
		}
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return fmt.Errorf("decode asciicast event: %w", err)
		}
		if len(parts) != 3 {
			return fmt.Errorf("invalid asciicast event: expected 3 fields, got %d", len(parts))
		}
		var offset float64
		var typ, data string
		if err := json.Unmarshal(parts[0], &offset); err != nil {
			return fmt.Errorf("decode asciicast event offset: %w", err)
		}
		if err := json.Unmarshal(parts[1], &typ); err != nil {
			return fmt.Errorf("decode asciicast event type: %w", err)
		}
		if err := json.Unmarshal(parts[2], &data); err != nil {
			return fmt.Errorf("decode asciicast event data: %w", err)
		}
		if typ != "i" && typ != "o" && typ != "r" {
			return fmt.Errorf("unsupported asciicast event type %q", typ)
		}
		stop, err := fn(terminalAuditCastEvent{offsetMS: int64(offset*1000 + 0.5), typ: typ, data: data})
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

func commandIndexAt(commands []TerminalAuditCommandEvidence, offsetMS int64) int {
	index := -1
	for i := range commands {
		if commands[i].TimeOffsetMS > offsetMS {
			break
		}
		index = i
	}
	return index
}

func detectOpaqueExecution(command string) (string, bool) {
	segments := strings.Split(command, "|")
	for _, segment := range segments[1:] {
		if isShellInterpreter(firstCommandExecutable(strings.Fields(segment))) {
			return "remote_script_content_unavailable", true
		}
	}

	fields := strings.Fields(segments[0])
	first := firstCommandExecutable(fields)
	if first == "" {
		return "", false
	}
	if first == "source" || first == "." {
		if len(fields) > 1 {
			return "script_content_unavailable", true
		}
		return "", false
	}
	if isShellInterpreter(first) {
		for i, field := range fields[1:] {
			if field == "-c" || field == "-e" {
				if i+2 < len(fields) && strings.HasPrefix(strings.Trim(fields[i+2], "'\""), "$") {
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
	if isScriptPath(first) {
		return "script_content_unavailable", true
	}
	return "", false
}

func firstCommandExecutable(fields []string) string {
	prefixOptions := false
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if field == "env" || field == "sudo" {
			prefixOptions = true
			continue
		}
		if strings.Contains(field, "=") {
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
		return strings.TrimSpace(field)
	}
	return ""
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
