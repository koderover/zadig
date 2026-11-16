package service

import (
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"

	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func missingAnswerFailure(choice model.Choice, lang language.Language) string {
	if choice.FinishReason != nil {
		switch *choice.FinishReason {
		case "length":
			return lang.Text("模型输出已达到长度上限，未能生成完整回答，请简化问题后重试", "The model reached its output limit without a complete answer. Simplify your request and try again.")
		case "content_filter":
			return lang.Text("模型响应被内容过滤，未能生成回答，请调整问题后重试", "The model response was filtered. Rephrase your request and try again.")
		}
	}
	if len(choice.Message.ToolCalls) > 0 {
		return lang.Text("工具调用后模型未生成最终回答，请重试", "The model did not produce a final answer after calling tools. Please try again.")
	}
	if strings.TrimSpace(choice.Message.ReasoningContent) != "" {
		return lang.Text("模型仅返回了推理内容，未生成回答，请重试", "The model returned only reasoning without an answer. Please try again.")
	}
	return lang.Text("模型未返回回答内容，请重试", "The model returned no answer. Please try again.")
}
