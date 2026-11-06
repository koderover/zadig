package models

// AIAssistantUsageReportRow 仅用于聚合查询，分组键由各报告转换为明确的维度字段。
type AIAssistantUsageReportRow struct {
	Key string
	AIAssistantStatsMetrics
}

type AIAssistantStatsMetrics struct {
	RunCount           int64    `json:"run_count"`
	Completed          int64    `json:"completed_count"`
	Terminal           int64    `json:"terminal_count"`
	InputTokens        int64    `json:"input_tokens"`
	OutputTokens       int64    `json:"output_tokens"`
	ElapsedMS          int64    `json:"elapsed_ms"`
	CompletedElapsedMS int64    `json:"completed_elapsed_ms"`
	Positive           int64    `json:"positive_count"`
	Negative           int64    `json:"negative_count"`
	ActiveUsers        int64    `json:"active_users"`
	CompletionRate     *float64 `json:"completion_rate" extensions:"x-nullable"`
	AdoptionRate       *float64 `json:"adoption_rate" extensions:"x-nullable"`
}

type AIAssistantStatsPeriod struct {
	From     string `json:"from"`      // 开始日期，包含当日。
	To       string `json:"to"`        // 结束日期，包含当日。
	TimeZone string `json:"time_zone"` // 与日聚合写入一致的部署时区。
}

type AIAssistantStatsOverviewResponse struct {
	AIAssistantStatsPeriod
	Metrics                  AIAssistantStatsMetrics `json:"metrics"`
	Estimation               bool                    `json:"estimation"` // 节省人力为估算值。
	EstimatedSavedMinutes    float64                 `json:"estimated_saved_minutes"`
	EstimatedSavedPersonDays *float64                `json:"estimated_saved_person_days" extensions:"x-nullable"` // 无反馈时为空。
}

type AIAssistantStatsTrendItem struct {
	Date string `json:"date"` // 日期，格式 YYYY-MM-DD。
	AIAssistantStatsMetrics
}

type AIAssistantStatsTrendsResponse struct {
	AIAssistantStatsPeriod
	Items []AIAssistantStatsTrendItem `json:"items"` // 按日期升序，仅返回有聚合记录的日期。
}

type AIAssistantStatsSkillItem struct {
	Category string `json:"category"` // Skill 分类：发布辅助、故障排查、实施辅助、巡检诊断。
	AIAssistantStatsMetrics
}

type AIAssistantStatsSkillsResponse struct {
	AIAssistantStatsPeriod
	Items []AIAssistantStatsSkillItem `json:"items"` // 按发布辅助、故障排查、实施辅助、巡检诊断排序，无数据的分类返回零计数与空比例。
}

type AIAssistantStatsRankingItem struct {
	UserID string `json:"user_id"`
	AIAssistantStatsMetrics
}

type AIAssistantStatsRankingsResponse struct {
	AIAssistantStatsPeriod
	Items []AIAssistantStatsRankingItem `json:"items"` // 最多二十条，按完成任务数、运行数降序，用户 ID 升序。
}
