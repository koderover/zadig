package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/language"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const maxConsecutiveToolErrors = 3

func updateToolErrorTarget(e *execution, raw []byte) {
	var args map[string]json.RawMessage
	if json.Unmarshal(raw, &args) != nil {
		return
	}
	if target, ok := args["target"]; ok {
		updateToolErrorTarget(e, target)
	}
	// 只认用户明确给出的资源身份；猜测的名称、分支和路径不能重置计数。
	for _, key := range []string{"project_key", "env_name", "service_name", "workflow_name", "repo_owner", "repo_name", "branch", "path"} {
		var value string
		if json.Unmarshal(args[key], &value) != nil || value == "" {
			continue
		}
		// 完整名称才算明确提及，避免把 repo 当作 repo-one 中的另一个目标。
		mentioned := regexp.MustCompile(`(^|[^a-zA-Z0-9_.-])` + regexp.QuoteMeta(value) + `($|[^a-zA-Z0-9_.-])`)
		if !mentioned.MatchString(e.Message) {
			continue
		}
		if previous := e.ToolErrorTarget[key]; previous != "" && previous != value {
			e.ToolErrorCount = 0
			e.ToolErrorTarget = nil
		}
		if e.ToolErrorTarget == nil {
			e.ToolErrorTarget = make(map[string]string)
		}
		e.ToolErrorTarget[key] = value
	}
}

func stopAfterToolFailures(ctx context.Context, args *tool.BeforeToolArgs) (*tool.BeforeToolResult, error) {
	if e, ok := ctx.Value(executionKey{}).(*execution); ok {
		if args != nil {
			updateToolErrorTarget(e, args.Arguments)
		}
		if e.ToolErrorCount < maxConsecutiveToolErrors {
			return nil, nil
		}
		return &tool.BeforeToolResult{CustomResult: map[string]any{
			"status":  "clarification_required",
			"message": "围绕同一目标连续查询报错三次，请停止猜测，根据已有错误说明原因并请求必要信息",
		}}, nil
	}
	return nil, nil
}

func applyToolFailureLimit(e *execution, req *model.Request) {
	if e.ToolErrorCount < maxConsecutiveToolErrors {
		return
	}
	req.Tools = nil
	req.Messages = append([]model.Message{model.NewSystemMessage("围绕同一目标已连续查询报错三次，停止工具调用，不再猜测名称、参数、路径或更换查询方式。根据已返回的错误和证据说明具体失败原因；无法确定原因时明确说明，不编造。请求用户补充解决问题所需的最少信息；权限或服务故障则说明需要恢复的条件，不索要凭据。不将失败当作资源不存在，不报告任务完成。")}, req.Messages...)
}

func recordToolFailure(ctx context.Context, args *tool.AfterToolArgs) (*tool.AfterToolResult, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		return nil, nil
	}
	updateToolErrorTarget(e, args.Arguments)
	// 同一批剩余调用被拦截后，保留触发停止的计数和原始错误。
	if e.ToolErrorCount >= maxConsecutiveToolErrors {
		return nil, nil
	}
	if args.Error == nil {
		e.ToolErrorCount = 0
		delete(e.ToolFailures, args.ToolName)
		return nil, nil
	}
	e.ToolErrorCount++
	message := args.Error.Error()
	var platformErr *PlatformError
	if errors.As(args.Error, &platformErr) {
		message = platformErr.UserMessage(string(e.Language))
	}
	if e.ToolFailures == nil {
		e.ToolFailures = make(map[string]string)
	}
	e.ToolFailures[args.ToolName] = redactText(message)
	return nil, nil
}

func incompleteSkillFailure(e *execution, lang language.Language) string {
	if len(e.ToolFailures) == 0 {
		return lang.Text("任务未完成：未取得所需查询结果或操作结果", "The task is incomplete: required query or operation results were not obtained")
	}
	names := make([]string, 0, len(e.ToolFailures))
	for name := range e.ToolFailures {
		names = append(names, name)
	}
	sort.Strings(names)
	messages := make([]string, 0, len(names))
	for _, name := range names {
		messages = append(messages, name+": "+e.ToolFailures[name])
	}
	message := []rune(lang.Text("任务未完成：", "The task is incomplete: ") + strings.Join(messages, "; "))
	// ai_run.error 最多保存 512 个字符。
	if len(message) > 512 {
		message = append(message[:511], '…')
	}
	return string(message)
}
