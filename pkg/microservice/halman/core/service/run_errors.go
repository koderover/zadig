package service

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/language"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const modelTimeoutError = "model_timeout"

var callLimitMessage = regexp.MustCompile(`^max (tool iterations|LLM calls) \([0-9]+\) exceeded$`)

func publicRunError(original *model.ResponseError, e *execution) *model.ResponseError {
	if original == nil {
		return nil
	}
	detail := redactText(original.Message)
	if e.OnboardingDraft != nil {
		for _, item := range e.OnboardingDraft.Services {
			detail = hideCredentialValues(detail, item.Variables, item.CredentialVariables)
		}
		for _, item := range e.OnboardingDraft.Builds {
			detail = hideCredentialValues(detail, item.Variables, item.CredentialVariables)
		}
	}
	runes := []rune(detail)
	if len(runes) > 512 {
		detail = string(runes[:512])
	}
	slog.Warn("AI 运行返回错误", "run_id", e.Run.ID, "error_type", redactText(original.Type), "error_detail", detail)
	lang := e.Language
	if original.Type == modelTimeoutError ||
		((original.Type == model.ErrorTypeStreamError || original.Type == model.ErrorTypeAPIError) && strings.Contains(original.Message, context.DeadlineExceeded.Error())) {
		return &model.ResponseError{Type: modelTimeoutError, Message: lang.Text("模型响应超时，请继续当前对话重试", "The model response timed out. Retry in this conversation.")}
	}
	if original.Type == "tool_iteration_limit" || original.Type == "llm_call_limit" ||
		((original.Type == model.ErrorTypeFlowError || original.Type == agent.ErrorTypeStopAgentError) && (callLimitMessage.MatchString(original.Message) || original.Message == "tool calls are disabled during call limit finalization")) {
		kind := "tool_iteration_limit"
		if original.Type == "llm_call_limit" || strings.Contains(original.Message, "LLM calls") {
			kind = "llm_call_limit"
		}
		message := lang.Text("本轮处理已达上限，请继续当前对话或缩小本轮任务范围", "This turn reached its processing limit. Continue this conversation or narrow the task.")
		if e.OnboardingDraft != nil {
			message = lang.Text("本轮处理已达上限，草稿已保存，可以继续补充信息", "This turn reached its processing limit. Your draft is saved; you can continue supplying information.")
		}
		return &model.ResponseError{Type: kind, Message: message}
	}
	if original.Type == model.ErrorTypeFlowError || original.Type == "agent_callback_error" || original.Type == "execution_error" {
		return &model.ResponseError{Type: "execution_error", Message: lang.Text("任务处理失败，请查看运行状态后重试", "Task processing failed. Check the run status and try again.")}
	}
	return &model.ResponseError{Type: "model_service_error", Message: lang.Text("模型服务调用失败，请稍后重试", "The model service call failed. Please try again later.")}
}

func hideCredentialValues(text string, values map[string]string, keys []string) string {
	for _, key := range keys {
		if value := values[key]; value != "" {
			text = strings.ReplaceAll(text, value, "[已隐藏]")
		}
	}
	return text
}

func callLimitFinalizationInstruction(locale string) string {
	return language.Normalize(locale).Text(
		"这是本轮最后一次回答，不再调用工具。根据已有信息直接回答；如任务未完成，简短说明当前进度、阻塞原因和下一步需要的信息。只在工具已成功保存草稿时说明草稿已保存。未执行的动作不能报告成功；已有待确认清单仍须用户点击确认卡片。不要要求用户重复提供草稿已有的信息，不展示凭据值。",
		"This is the final response for this turn. Do not call more tools. Answer from the available information. If the task is incomplete, briefly explain the progress, the blocker and what is needed next. Say the draft is saved only if a tool successfully saved it. Do not claim unexecuted actions succeeded. Existing pending plans still require the confirmation card. Do not ask again for saved information or display credential values.",
	)
}

// 业务参数错误作为结果返回，避免框架在错误日志中输出包含凭据的原始工具参数。
type privateOnboardingTool struct{ tool.CallableTool }

type onboardingToolFailure struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (t privateOnboardingTool) Call(ctx context.Context, args []byte) (any, error) {
	result, err := t.CallableTool.Call(ctx, args)
	if err == nil {
		return result, nil
	}
	message := redactText(err.Error())
	if e, ok := ctx.Value(executionKey{}).(*execution); ok && e.OnboardingDraft != nil {
		for _, item := range e.OnboardingDraft.Services {
			message = hideCredentialValues(message, item.Variables, item.CredentialVariables)
		}
		for _, item := range e.OnboardingDraft.Builds {
			message = hideCredentialValues(message, item.Variables, item.CredentialVariables)
		}
	}
	return &onboardingToolFailure{Status: "validation_failed", Message: message}, nil
}
