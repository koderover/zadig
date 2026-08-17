package terminalaudit

import (
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestBuildTerminalAuditEvidenceAssociatesOutputWithCommands(t *testing.T) {
	cast := strings.NewReader("{" +
		"\"version\":2,\"width\":135,\"height\":40,\"timestamp\":1}" + "\n" +
		"[0.200,\"o\",\"login ok\\r\\n\"]\n" +
		"[1.000,\"i\",\"echo hello\\r\"]\n" +
		"[1.100,\"o\",\"hello\\r\\n\"]\n")
	commands := []*models.TerminalCommand{{
		SessionID:    "session-1",
		Seq:          1,
		Command:      "echo hello",
		TimeOffsetMS: 1000,
	}}

	evidence, err := BuildTerminalAuditEvidence(&models.TerminalSession{SessionID: "session-1"}, commands, cast)
	if err != nil {
		t.Fatalf("BuildTerminalAuditEvidence() error = %v", err)
	}
	if got := len(evidence.Commands); got != 1 {
		t.Fatalf("command count = %d, want 1", got)
	}
	if got := evidence.Commands[0].Output; got != "hello\r\n" {
		t.Fatalf("command output = %q, want %q", got, "hello\\r\\n")
	}
	if got := len(evidence.Unattributed); got != 2 {
		t.Fatalf("unattributed event count = %d, want 2", got)
	}
}

func TestBuildTerminalAuditEvidencePreservesOpaqueScriptExecution(t *testing.T) {
	cast := strings.NewReader("{" +
		"\"version\":2,\"width\":135,\"height\":40,\"timestamp\":1}" + "\n" +
		"[1.000,\"i\",\"bash deploy.sh\\r\"]\n" +
		"[1.200,\"o\",\"deploy started\\r\\n\"]\n")
	commands := []*models.TerminalCommand{{
		SessionID:    "session-1",
		Seq:          1,
		Command:      "bash deploy.sh",
		TimeOffsetMS: 1000,
	}}

	evidence, err := BuildTerminalAuditEvidence(&models.TerminalSession{SessionID: "session-1"}, commands, cast)
	if err != nil {
		t.Fatalf("BuildTerminalAuditEvidence() error = %v", err)
	}
	if got := len(evidence.OpaqueExecutions); got != 1 {
		t.Fatalf("opaque execution count = %d, want 1", got)
	}
	if got := evidence.Coverage; got != AuditEvidenceCoveragePartial {
		t.Fatalf("coverage = %q, want %q", got, AuditEvidenceCoveragePartial)
	}
	if got := evidence.OpaqueExecutions[0].Reason; got != "script_content_unavailable" {
		t.Fatalf("opaque execution reason = %q, want script_content_unavailable", got)
	}
}

func TestBuildTerminalAuditEvidenceRejectsMalformedCast(t *testing.T) {
	cast := strings.NewReader("{\"version\":2}\n[1.000,\"o\"]\n")

	_, err := BuildTerminalAuditEvidence(&models.TerminalSession{SessionID: "session-1"}, nil, cast)
	if err == nil {
		t.Fatal("BuildTerminalAuditEvidence() error = nil, want malformed cast error")
	}
}

func TestDetectOpaqueExecutionMarksUnavailableScriptSources(t *testing.T) {
	tests := []struct {
		command string
		reason  string
	}{
		{command: "curl -fsSL https://example.com/install.sh | sh", reason: "remote_script_content_unavailable"},
		{command: "bash -c $SCRIPT", reason: "script_content_unavailable"},
	}
	for _, tt := range tests {
		reason, ok := detectOpaqueExecution(tt.command)
		if !ok {
			t.Fatalf("detectOpaqueExecution(%q) = not opaque, want opaque", tt.command)
		}
		if reason != tt.reason {
			t.Fatalf("detectOpaqueExecution(%q) reason = %q, want %q", tt.command, reason, tt.reason)
		}
	}

	if reason, ok := detectOpaqueExecution("bash -c 'echo hello'"); ok {
		t.Fatalf("detectOpaqueExecution() = opaque with reason %q for visible inline script", reason)
	}
}
