package terminalaudit

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestBuildTerminalAuditEvidenceFromCommands(t *testing.T) {
	session := &models.TerminalSession{
		SessionID:   "session-1",
		SessionType: models.TerminalSessionTypePodExec,
		Username:    "user-1",
		ProjectName: "project-1",
	}
	commands := []*models.TerminalCommand{
		{Seq: 1, Command: "kubectl get pods", TimeOffsetMS: 1000},
		{Seq: 2, Command: "bash deploy.sh", TimeOffsetMS: 2000},
	}

	evidence := BuildTerminalAuditEvidence(session, commands)

	if evidence.Session.SessionID != session.SessionID || evidence.Session.ProjectName != session.ProjectName {
		t.Fatalf("unexpected session evidence: %+v", evidence.Session)
	}
	if len(evidence.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(evidence.Commands))
	}
	if evidence.Commands[0].Command != commands[0].Command || evidence.Commands[0].TimeOffsetMS != commands[0].TimeOffsetMS {
		t.Fatalf("unexpected first command evidence: %+v", evidence.Commands[0])
	}
	if evidence.Commands[1].OpaqueExecution != "script_content_unavailable" {
		t.Fatalf("expected opaque script marker, got %q", evidence.Commands[1].OpaqueExecution)
	}
	if evidence.Coverage != AuditEvidenceCoveragePartial {
		t.Fatalf("expected partial coverage, got %q", evidence.Coverage)
	}
}
