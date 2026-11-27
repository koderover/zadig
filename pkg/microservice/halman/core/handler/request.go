package handler

import "encoding/json"

type RequestIDRequest struct {
	// 新请求标识；同一请求重复提交时保持不变。
	RequestID string `json:"request_id" validate:"required" example:"chat-20261103-001"`
}

// 保留字段存在性，以拒绝旧客户端传来的空值和 null。
type modelOverrides struct {
	LLMIntegrationID json.RawMessage `json:"llm_integration_id"`
	ReasoningEffort  json.RawMessage `json:"reasoning_effort"`
}

func (o modelOverrides) present() bool {
	return o.LLMIntegrationID != nil || o.ReasoningEffort != nil
}

const modelOverrideMessage = "请通过 /api/halman/preferences 修改模型或思考强度"

type ConfirmationRequest struct {
	// 确认卡片中的计划版本，必须大于零。
	Version int `json:"version" validate:"required" minimum:"1" example:"1"`
	// 为空或 confirm 表示确认，reject 表示拒绝。
	Decision string `json:"decision" example:"confirm"`
}

type FeedbackRequest struct {
	// 被评价的回答版本，必须大于零。
	Version int `json:"answer_version" validate:"required" minimum:"1" example:"1"`
	// 是否有帮助；false 也是有效反馈，不能省略。
	Helpful *bool `json:"helpful" validate:"required" example:"true"`
}

type ReconcileRequest struct {
	// 需要只读核实的计划版本，必须大于零。
	Version int `json:"version" validate:"required" minimum:"1" example:"1"`
}
