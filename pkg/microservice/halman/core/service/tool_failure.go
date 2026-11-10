package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/language"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func recordToolFailure(ctx context.Context, args *tool.AfterToolArgs) (*tool.AfterToolResult, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		return nil, nil
	}
	if args.Error == nil {
		delete(e.ToolFailures, args.ToolName)
		return nil, nil
	}
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
