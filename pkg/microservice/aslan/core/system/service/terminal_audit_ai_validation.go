package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/terminalaudit"
)

type terminalAuditAIRedactor struct {
	pattern     *regexp.Regexp
	replacement string
}

var terminalAuditAIRedactors = []terminalAuditAIRedactor{
	{regexp.MustCompile(`(?is)-----BEGIN [^-\r\n]*PRIVATE KEY-----.*?(?:-----END [^-\r\n]*PRIVATE KEY-----|$)`), `[REDACTED PRIVATE KEY]`},
	{regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|basic)\s+)[^\s'"]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(cookie\s*:\s*)[^\r\n'"]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(["']?(?:api[_-]?(?:key|token)|access[_-]?token|password|passwd|token|secret)["']?\s*[:=]\s*)("[^"]*(?:"|$)|'[^']*(?:'|$)|[^\s,;&'"]+)`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(--(?:password|passwd|token|secret|api[_-]?key)(?:=|\s+))("[^"]*"|'[^']*'|[^\s]+)`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(\bcurl\b[^\r\n]*?\s(?:-u|--user)(?:=|\s+)["']?[^:\s"']+:)([^@\s"']+)`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(https?://[^/\s:@]+:)[^@\s/]+@`), `${1}[REDACTED]@`},
}

func sanitizeTerminalAuditEvidenceForAI(evidence *terminalaudit.TerminalAuditEvidence) {
	sessionFields := []*string{
		&evidence.Session.SessionID, &evidence.Session.Username, &evidence.Session.Account,
		&evidence.Session.ProjectName, &evidence.Session.EnvName, &evidence.Session.ServiceName,
		&evidence.Session.WorkflowName, &evidence.Session.JobName, &evidence.Session.TargetName,
		&evidence.Session.Protocol, &evidence.Session.RemoteAddr, &evidence.Session.LoginAccount,
		&evidence.Session.HostName, &evidence.Session.HostIP, &evidence.Session.Namespace,
		&evidence.Session.PodName, &evidence.Session.ContainerName,
	}
	for _, field := range sessionFields {
		*field = redactTerminalAuditAISecrets(*field)
	}
	for i := range evidence.Commands {
		evidence.Commands[i].Command = redactTerminalAuditAISecrets(evidence.Commands[i].Command)
		evidence.Commands[i].Output = redactTerminalAuditAISecrets(evidence.Commands[i].Output)
	}
	for i := range evidence.Unattributed {
		evidence.Unattributed[i].Data = redactTerminalAuditAISecrets(evidence.Unattributed[i].Data)
	}
}

func redactTerminalAuditAISecrets(value string) string {
	for _, redactor := range terminalAuditAIRedactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	return value
}

type terminalAuditAIAnswer struct {
	RiskLevel string                                `json:"risk_level"`
	Findings  []commonmodels.TerminalAuditAIFinding `json:"findings"`
}

type terminalAuditAIChunk struct {
	evidence    string
	commands    map[int64]string
	serialGroup int
}

func parseAndValidateTerminalAuditAIAnswer(answer string, commands map[int64]string) (*terminalAuditAIAnswer, error) {
	parsed := new(terminalAuditAIAnswer)
	if err := json.Unmarshal([]byte(llmservice.ExtractJSONCodeBlock(answer)), parsed); err != nil {
		return nil, fmt.Errorf("decode ai answer json: %w", err)
	}
	if parsed.Findings == nil {
		return nil, errors.New("ai answer findings are required")
	}
	switch parsed.RiskLevel {
	case "low", "medium", "high":
	default:
		return nil, fmt.Errorf("invalid risk_level %q, want low, medium or high", parsed.RiskLevel)
	}
	if parsed.RiskLevel != "low" && len(parsed.Findings) == 0 {
		return nil, fmt.Errorf("risk_level %s requires at least one finding", parsed.RiskLevel)
	}

	for i := range parsed.Findings {
		finding := &parsed.Findings[i]
		command, ok := commands[finding.Seq]
		if !ok {
			return nil, fmt.Errorf("finding references unknown command seq %d", finding.Seq)
		}
		finding.Risk = strings.TrimSpace(finding.Risk)
		finding.Reason = strings.TrimSpace(finding.Reason)
		finding.Suggestion = strings.TrimSpace(finding.Suggestion)
		if finding.Risk == "" || finding.Reason == "" || finding.Suggestion == "" {
			return nil, fmt.Errorf("finding for command seq %d has an empty risk, reason or suggestion", finding.Seq)
		}
		finding.Command = command
	}
	return parsed, nil
}
