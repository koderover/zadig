package models

// AIAssistantUsageReportRow 仅用于聚合查询，分组键由各报告转换为明确的维度字段。
type AIAssistantUsageReportRow struct {
	Key string
	AIAssistantStatsMetrics
}

type AIAssistantStatsModelUsage struct {
	ModelID      string `json:"model_id"` // 模型集成 ID。
	Model        string `json:"model"`    // 调用时的模型名称。
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

type AIAssistantStatsMetrics struct {
	ModelUsage         []AIAssistantStatsModelUsage `json:"model_usage"` // 各模型实际用量，无数据时返回空数组。
	TotalTokens        int64                        `json:"total_tokens"`
	RunCount           int64                        `json:"run_count"`
	Completed          int64                        `json:"completed_count"`
	Terminal           int64                        `json:"terminal_count"`
	InputTokens        int64                        `json:"input_tokens"`
	OutputTokens       int64                        `json:"output_tokens"`
	ElapsedMS          int64                        `json:"elapsed_ms"`
	CompletedElapsedMS int64                        `json:"completed_elapsed_ms"`
	Positive           int64                        `json:"positive_count"`
	Negative           int64                        `json:"negative_count"`
	ActiveUsers        int64                        `json:"active_users"`
	CompletionRate     *float64                     `json:"completion_rate" extensions:"x-nullable"`
	AdoptionRate       *float64                     `json:"adoption_rate" extensions:"x-nullable"`
}

type AIAssistantStatsPeriod struct {
	From     string `json:"from"`      // 开始日期，包含当日。
	To       string `json:"to"`        // 结束日期，包含当日。
	TimeZone string `json:"time_zone"` // 与日聚合写入一致的部署时区。
}

type AIAssistantStatsOverviewData struct {
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
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"` // 当前用户名；用户已删除时回退为用户 ID。
	AIAssistantStatsMetrics
}

type AIAssistantStatsRankingsResponse struct {
	AIAssistantStatsPeriod
	Items []AIAssistantStatsRankingItem `json:"items"` // 最多二十条，按完成任务数、运行数降序，用户 ID 升序。
}

type AIAssistantStatsOverviewResponse struct {
	AIAssistantStatsOverviewData
	Previous   *AIAssistantStatsOverviewData `json:"previous" extensions:"x-nullable"` // 紧邻的等长上期；超出聚合保留范围时为 null。
	TotalUsers int64                         `json:"total_users"`                      // 查询时的系统用户总数，非历史用户数。
}
