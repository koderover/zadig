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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	maxTerminalAuditAICommands       = 100
	maxTerminalAuditAICommandRunes   = 500
	maxTerminalAuditAIOutputRunes    = 1000
	maxTerminalAuditAIPromptBytes    = 48 * 1024
	terminalAuditAIOutputTruncated   = "\n...[output truncated]...\n"
	terminalAuditAICommandsTruncated = "\n...[%d commands omitted]...\n"
)
const terminalAuditAIPrompt = `你是一名终端命令安全审查专员。请根据提供的终端会话审计数据，识别命令安全风险，并给出可执行的整改建议。

审查要求：
1. 只根据提供的命令和输出判断，不要臆测未提供的内容。
2. 当 opaque_executions 非空时，说明对应脚本内容未被审计到，必须将该命令标记为"脚本内容不可审计"，不得假设脚本行为。
3. 重点关注：远程脚本下载后执行、敏感文件读取、密钥/凭证泄露、权限提升、破坏性操作、可疑网络传输（curl/scp/rsync 等）。
4. 风险定级只能使用 low、medium、high。没有风险时 findings 可以为空数组。
5. 最终只输出一个 JSON 对象，不要输出 markdown 代码块或任何额外说明。格式：
{"risk_level":"low|medium|high","summary":"整体结论","findings":[{"seq":命令序号,"command":"命令","risk":"风险类型","reason":"判断依据","suggestion":"整改建议"}]}

会话元数据：
%s

不可完整审计的执行：
%s

命令列表：
%s`

// AnalyzeTerminalSession rebuilds terminal audit evidence from the stored cast
// file and asks the configured LLM to review it as a command security auditor.
// The result is persisted and returned so the frontend can render it directly.
func AnalyzeTerminalSession(sessionID string) (*commonmodels.TerminalAuditAIResult, error) {
	session, err := terminalaudit.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session.Status == commonmodels.TerminalSessionStatusRunning {
		return nil, e.NewWithDesc(e.ErrInvalidParam, "terminal session is still running")
	}
	if session.ObjectKey == "" {
		return nil, e.NewWithDesc(e.ErrNotFound, "terminal cast file is not available")
	}

	commands, _, err := commonrepo.NewTerminalCommandColl().List(&commonmodels.TerminalCommandListArgs{SessionID: sessionID})
	if err != nil {
		return nil, fmt.Errorf("list terminal commands: %w", err)
	}

	stream, err := terminalaudit.GetCastStream(sessionID)
	if err != nil {
		return nil, err
	}
	defer stream.Body.Close()

	evidence, err := terminalaudit.BuildTerminalAuditEvidence(session, commands, stream.Body)
	if err != nil {
		return nil, fmt.Errorf("build terminal audit evidence: %w", err)
	}

	prompt, err := buildTerminalAuditAIPrompt(evidence)
	if err != nil {
		return nil, fmt.Errorf("build terminal audit ai prompt: %w", err)
	}

	ctx := context.Background()
	client, err := commonservice.GetDefaultLLMClient(ctx)
	if err != nil {
		return nil, err
	}

	options := []llm.ParamOption{llm.WithTemperature(0.1)}
	if model := client.GetModel(); model != "" {
		options = append(options, llm.WithModel(model))
	}
	answer, err := client.GetCompletion(ctx, prompt, options...)
	if err != nil {
		result := newTerminalAuditAIFailure(sessionID, evidence, prompt, "", 0, err)
		_ = commonrepo.NewTerminalAuditAIResultColl().Upsert(result)
		return nil, fmt.Errorf("analyze terminal session with ai: %w", err)
	}

	tokenNum := 0
	if num, tokenErr := llm.NumTokensFromPrompt(prompt, client.GetModel()); tokenErr == nil {
		tokenNum = num
	}

	parsed, err := parseTerminalAuditAIAnswer(answer)
	if err != nil {
		result := newTerminalAuditAIFailure(sessionID, evidence, prompt, answer, tokenNum, err)
		_ = commonrepo.NewTerminalAuditAIResultColl().Upsert(result)
		return nil, fmt.Errorf("parse terminal audit ai answer: %w", err)
	}

	result := &commonmodels.TerminalAuditAIResult{
		SessionID: sessionID,
		Status:    commonmodels.TerminalAuditAIStatusSucceeded,
		RiskLevel: parsed.RiskLevel,
		Summary:   parsed.Summary,
		Findings:  parsed.Findings,
		Coverage:  string(evidence.Coverage),
		Prompt:    prompt,
		Answer:    answer,
		TokenNum:  tokenNum,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := commonrepo.NewTerminalAuditAIResultColl().Upsert(result); err != nil {
		return nil, fmt.Errorf("save terminal audit ai result: %w", err)
	}
	return result, nil
}

func GetTerminalSessionAIResult(sessionID string) (*commonmodels.TerminalAuditAIResult, error) {
	result, err := commonrepo.NewTerminalAuditAIResultColl().FindBySessionID(sessionID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, e.NewWithDesc(e.ErrNotFound, "terminal session ai audit result not found")
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
func buildTerminalAuditAIPrompt(evidence *terminalaudit.TerminalAuditEvidence) (string, error) {
	if evidence == nil {
		return "", fmt.Errorf("terminal audit evidence is nil")
	}
	meta, err := json.Marshal(evidence.Session)
	if err != nil {
		return "", err
	}
	opaque, err := json.Marshal(evidence.OpaqueExecutions)
	if err != nil {
		return "", err
	}

	var commands strings.Builder
	included := 0
	omitted := 0
	for _, command := range evidence.Commands {
		if included >= maxTerminalAuditAICommands {
			omitted = len(evidence.Commands) - included
			break
		}
		entry := fmt.Sprintf("\n[seq=%d offset_ms=%d]\n%s\n输出：\n%s\n",
			command.Seq,
			command.TimeOffsetMS,
			truncateRunes(command.Command, maxTerminalAuditAICommandRunes),
			truncateRunes(command.Output, maxTerminalAuditAIOutputRunes))
		if commands.Len()+len(entry) > maxTerminalAuditAIPromptBytes {
			omitted = len(evidence.Commands) - included
			break
		}
		commands.WriteString(entry)
		included++
	}
	if omitted > 0 {
		commands.WriteString(fmt.Sprintf(terminalAuditAICommandsTruncated, omitted))
	}
	if evidence.Unattributed != nil {
		commands.WriteString(fmt.Sprintf("\n未归属的原始终端事件：%d 条。\n", len(evidence.Unattributed)))
	}
	return fmt.Sprintf(terminalAuditAIPrompt, string(meta), string(opaque), commands.String()), nil
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + terminalAuditAIOutputTruncated
}

type terminalAuditAIAnswer struct {
	RiskLevel string                                `json:"risk_level"`
	Summary   string                                `json:"summary"`
	Findings  []commonmodels.TerminalAuditAIFinding `json:"findings"`
}

func parseTerminalAuditAIAnswer(answer string) (*terminalAuditAIAnswer, error) {
	cleaned := strings.TrimSpace(answer)
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(cleaned, "```json"), "```"))
	}
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("ai answer does not contain a json object")
	}
	parsed := &terminalAuditAIAnswer{}
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), parsed); err != nil {
		return nil, fmt.Errorf("decode ai answer json: %w", err)
	}
	switch parsed.RiskLevel {
	case "low", "medium", "high":
	default:
		return nil, fmt.Errorf("invalid risk_level %q, want low, medium or high", parsed.RiskLevel)
	}
	if parsed.Findings == nil {
		parsed.Findings = make([]commonmodels.TerminalAuditAIFinding, 0)
	}
	return parsed, nil
}

func newTerminalAuditAIFailure(
	sessionID string,
	evidence *terminalaudit.TerminalAuditEvidence,
	prompt string,
	answer string,
	tokenNum int,
	err error,
) *commonmodels.TerminalAuditAIResult {
	coverage := ""
	if evidence != nil {
		coverage = string(evidence.Coverage)
	}
	return &commonmodels.TerminalAuditAIResult{
		SessionID:    sessionID,
		Status:       commonmodels.TerminalAuditAIStatusFailed,
		Coverage:     coverage,
		Prompt:       prompt,
		Answer:       answer,
		TokenNum:     tokenNum,
		ErrorMessage: err.Error(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
}
