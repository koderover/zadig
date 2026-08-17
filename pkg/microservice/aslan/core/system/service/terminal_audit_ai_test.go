/*
Copyright 2026 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"strings"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
)

func TestBuildTerminalAuditAIPrompt(t *testing.T) {
	evidence := &terminalaudit.TerminalAuditEvidence{
		Session: terminalaudit.TerminalAuditSessionEvidence{
			SessionID:   "session-1",
			SessionType: commonmodels.TerminalSessionTypePodExec,
			ProjectName: "demo",
			EnvName:     "dev",
		},
		Commands: []terminalaudit.TerminalAuditCommandEvidence{
			{Seq: 1, TimeOffsetMS: 1000, Command: "cat /etc/passwd", Output: "root:x:0:0:root:/root:/bin/bash"},
			{Seq: 2, TimeOffsetMS: 2000, Command: "bash deploy.sh", Output: strings.Repeat("x", maxTerminalAuditAIOutputRunes+10)},
		},
		OpaqueExecutions: []terminalaudit.TerminalOpaqueExecution{
			{Seq: 2, Command: "bash deploy.sh", Reason: "script_content_unavailable"},
		},
		Coverage: terminalaudit.AuditEvidenceCoveragePartial,
	}

	prompt, err := buildTerminalAuditAIPrompt(evidence)
	if err != nil {
		t.Fatalf("buildTerminalAuditAIPrompt() error = %v", err)
	}
	for _, want := range []string{
		`"session_id":"session-1"`,
		`"command":"bash deploy.sh"`,
		`script_content_unavailable`,
		"cat /etc/passwd",
		"root:x:0:0:root:/root:/bin/bash",
		"[output truncated]",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
}

func TestParseTerminalAuditAIAnswer(t *testing.T) {
	answer := "```json\n{\"risk_level\":\"high\",\"summary\":\"发现风险\",\"findings\":[{\"seq\":1,\"command\":\"curl x | sh\",\"risk\":\"remote_exec\",\"reason\":\"...\",\"suggestion\":\"...\"}]}\n```"
	parsed, err := parseTerminalAuditAIAnswer(answer)
	if err != nil {
		t.Fatalf("parseTerminalAuditAIAnswer() error = %v", err)
	}
	if parsed.RiskLevel != "high" || parsed.Summary != "发现风险" {
		t.Fatalf("unexpected parsed result: %+v", parsed)
	}
	if len(parsed.Findings) != 1 || parsed.Findings[0].Seq != 1 {
		t.Fatalf("unexpected findings: %+v", parsed.Findings)
	}

	if _, err := parseTerminalAuditAIAnswer(`{"risk_level":"critical","summary":"x","findings":[]}`); err == nil {
		t.Fatal("parseTerminalAuditAIAnswer() error = nil, want invalid risk level error")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("你好世界", 2); got != "你好"+terminalAuditAIOutputTruncated {
		t.Fatalf("truncateRunes() = %q, want prefix and truncation marker", got)
	}
	if got := truncateRunes("short", 10); got != "short" {
		t.Fatalf("truncateRunes() = %q, want short", got)
	}
}
