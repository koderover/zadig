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

package handler

// AIReviewStatsTimeRange selects reviews by Job completion time, in Unix seconds.
type AIReviewStatsTimeRange struct {
	StartTime int64 `form:"start_time" binding:"required,gt=0"`
	EndTime   int64 `form:"end_time" binding:"required,gtfield=StartTime"`
}

type AIReviewStatsOverviewRequest struct {
	AIReviewStatsTimeRange
	ProjectName string `form:"project_name"`
	CodehostID  *int   `form:"codehost_id" binding:"omitempty,gt=0"`
	RepoOwner   string `form:"repo_owner"`
	RepoName    string `form:"repo_name"`
}

type AIReviewStatsPagination struct {
	Page     int `form:"page,default=1" binding:"gte=1"`
	PageSize int `form:"page_size,default=20" binding:"gte=1,lte=100"`
}

type AIReviewStatsListRequest struct {
	AIReviewStatsTimeRange
	AIReviewStatsPagination
	SortBy    string `form:"sort_by,default=pr_count" binding:"oneof=pr_count inline_total resolution_rate up_down_ratio total_tokens"`
	SortOrder string `form:"sort_order,default=desc" binding:"oneof=asc desc"`
}

type AIReviewStatsRepoListRequest struct {
	AIReviewStatsListRequest
	ProjectName string `form:"project_name"`
}

type AIReviewStatsRepoPRListRequest struct {
	AIReviewStatsTimeRange
	AIReviewStatsPagination
	CodehostID  int    `form:"codehost_id" binding:"required,gt=0"`
	RepoOwner   string `form:"repo_owner" binding:"required"`
	ProjectName string `form:"project_name"`
}

type AIReviewStatsModelUsage struct {
	Model            string `json:"model"`             // 模型名称
	PromptTokens     int64  `json:"prompt_tokens"`     // 输入 Token 数量
	CompletionTokens int64  `json:"completion_tokens"` // 输出 Token 数量
	TotalTokens      int64  `json:"total_tokens"`      // Token 总量
}

type AIReviewStatsMetrics struct {
	PRCount          int64                     `json:"pr_count"`                                // 所选范围内去重后的审查 PR 数量
	InlineTotal      int64                     `json:"inline_total"`                            // 成功发布的 AI 行内问题总数，包含已删除线程
	InlineResolved   int64                     `json:"inline_resolved"`                         // 已解决的 AI 行内问题数量
	InlineUnresolved int64                     `json:"inline_unresolved"`                       // 已确认未解决的行内问题数量；解决状态未知时处理率为 null
	ResolutionRate   *float64                  `json:"resolution_rate" extensions:"x-nullable"` // 处理率，取值 0–1；无行内问题或解决状态未知时为 null
	FindingTotal     int64                     `json:"finding_total"`                           // 所选范围内按 PR 和 fingerprint 去重的问题数量；缺少 fingerprint 的问题分别计数
	Up               int64                     `json:"up"`                                      // 点赞数量
	Down             int64                     `json:"down"`                                    // 点踩数量
	UpDownRatio      *float64                  `json:"up_down_ratio" extensions:"x-nullable"`   // 赞踩比，点赞数/点踩数四舍五入保留一位小数，前端显示为 x:1；点踩数为零时取点赞数，反馈未知时为 null
	ApprovalRate     *float64                  `json:"approval_rate" extensions:"x-nullable"`   // 好评率，点赞数/反馈总数，取值 0–1；赞踩均为零时为 0，反馈未知时为 null
	PromptTokens     int64                     `json:"prompt_tokens"`                           // 输入 Token 数量
	CompletionTokens int64                     `json:"completion_tokens"`                       // 输出 Token 数量
	TotalTokens      int64                     `json:"total_tokens"`                            // Token 总量
	ModelUsage       []AIReviewStatsModelUsage `json:"model_usage"`                             // 各模型的 Token 用量
}

type AIReviewStatsDistributionItem struct {
	ID    string `json:"id"`    // 严重级别或问题类型的标识
	Name  string `json:"name"`  // 展示名称
	Count int64  `json:"count"` // 问题数量
}

type AIReviewStatsDistributions struct {
	Severities   []AIReviewStatsDistributionItem `json:"severities"`    // 严重级别分布，仅返回有数据的项
	ProblemTypes []AIReviewStatsDistributionItem `json:"problem_types"` // 问题类型分布，使用 category/category_name，返回全部有数据的类型
}

type AIReviewStatsComparison struct {
	PRCount        *float64 `json:"pr_count" extensions:"x-nullable"`        // 审查 PR 数量环比，计算为（本期-上期）/上期；上期为零时为 null
	ResolutionRate *float64 `json:"resolution_rate" extensions:"x-nullable"` // 处理率环比，单位为百分点，计算为（本期-上期）×100；无法计算时为 null
	UpDownRatio    *float64 `json:"up_down_ratio" extensions:"x-nullable"`   // 赞踩比环比，计算为本期减上期；无法计算时为 null
	TotalTokens    *float64 `json:"total_tokens" extensions:"x-nullable"`    // Token 总量环比，计算为（本期-上期）/上期；上期为零时为 null
}

type AIReviewStatsWeeklyTrend struct {
	StartTime      int64    `json:"start_time"`                              // 区间开始时间，Unix 秒时间戳，包含；从所选开始时间起每七天分桶
	EndTime        int64    `json:"end_time"`                                // 区间结束时间，Unix 秒时间戳，不包含；最后一个区间可不足七天
	PRCount        int64    `json:"pr_count"`                                // 所选范围内去重后的审查 PR 数量
	InlineTotal    int64    `json:"inline_total"`                            // 成功发布的 AI 行内问题总数，包含已删除线程
	InlineResolved int64    `json:"inline_resolved"`                         // 已解决的 AI 行内问题数量
	ResolutionRate *float64 `json:"resolution_rate" extensions:"x-nullable"` // 处理率，取值 0–1；无行内问题或解决状态未知时为 null
}

type AIReviewStatsScope struct {
	Type               string `json:"type" enums:"global,project,repo"` // 统计范围类型：global 全局、project 项目、repo 代码库
	ProjectName        string `json:"project_name"`                     // 项目标识
	ProjectDisplayName string `json:"project_display_name"`             // 项目展示名称
	CodehostID         int    `json:"codehost_id"`                      // 代码源 ID
	RepoOwner          string `json:"repo_owner"`                       // 仓库 owner，GitLab 使用完整 namespace
	RepoName           string `json:"repo_name"`                        // 仓库名称
	RepoDisplayName    string `json:"repo_display_name"`                // 代码库展示名称
	RepoCount          int64  `json:"repo_count"`                       // 代码库数量
}

type AIReviewStatsOverviewResponse struct {
	Scope                      AIReviewStatsScope         `json:"scope"`        // 统计范围及其身份信息
	Metrics                    AIReviewStatsMetrics       `json:"metrics"`      // 公共统计指标
	Comparison                 AIReviewStatsComparison    `json:"comparison"`   // 与紧邻当前期间的等长上一期间比较的环比数据
	WeeklyTrend                []AIReviewStatsWeeklyTrend `json:"weekly_trend"` // 按周统计的 PR 数量与处理率趋势，统计实现后包含空区间
	AIReviewStatsDistributions                            // 问题分布，字段展开到当前对象
}

type AIReviewStatsProjectItem struct {
	ProjectName                string `json:"project_name"`         // 项目标识
	ProjectDisplayName         string `json:"project_display_name"` // 项目展示名称
	RepoCount                  int64  `json:"repo_count"`           // 代码库数量
	AIReviewStatsMetrics              // 公共统计指标，字段展开到当前对象
	AIReviewStatsDistributions        // 问题分布，字段展开到当前对象
}

type AIReviewStatsRepoItem struct {
	CodehostID                 int    `json:"codehost_id"`       // 代码源 ID
	RepoOwner                  string `json:"repo_owner"`        // 仓库 owner，GitLab 使用完整 namespace
	RepoName                   string `json:"repo_name"`         // 仓库名称
	RepoDisplayName            string `json:"repo_display_name"` // 代码库展示名称
	AIReviewStatsMetrics              // 公共统计指标，字段展开到当前对象
	AIReviewStatsDistributions        // 问题分布，字段展开到当前对象
}

type AIReviewStatsPRItem struct {
	CodehostID                 int    `json:"codehost_id"`                                  // 代码源 ID
	RepoOwner                  string `json:"repo_owner"`                                   // 仓库 owner，GitLab 使用完整 namespace
	RepoName                   string `json:"repo_name"`                                    // 仓库名称
	PR                         int    `json:"pr"`                                           // PR 编号，GitLab 为 MR 的 IID
	PRTitle                    string `json:"pr_title"`                                     // PR/MR 标题
	PRURL                      string `json:"pr_url"`                                       // PR/MR 详情页链接
	PRAuthor                   string `json:"pr_author"`                                    // PR/MR 作者
	ReviewedAt                 int64  `json:"reviewed_at"`                                  // 所选范围内最后一次审查 Job 完成时间，Unix 秒时间戳
	ResolutionSyncedAt         *int64 `json:"resolution_synced_at" extensions:"x-nullable"` // 最近一次解决状态校对时间，Unix 秒时间戳；首次校对前为 null
	AIReviewStatsMetrics              // 公共统计指标，字段展开到当前对象
	AIReviewStatsDistributions        // 问题分布，字段展开到当前对象
}

type AIReviewStatsPage struct {
	Total    int64 `json:"total"`     // 符合筛选条件的记录总数
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页记录数量
}

type AIReviewStatsProjectListResponse struct {
	AIReviewStatsPage                            // 分页信息，字段展开到当前对象
	Items             []AIReviewStatsProjectItem `json:"items"` // 当前页的数据列表
}

type AIReviewStatsRepoListResponse struct {
	AIReviewStatsPage                         // 分页信息，字段展开到当前对象
	Items             []AIReviewStatsRepoItem `json:"items"` // 当前页的数据列表
}

type AIReviewStatsPRListResponse struct {
	AIReviewStatsPage                       // 分页信息，字段展开到当前对象
	Items             []AIReviewStatsPRItem `json:"items"` // 当前页的数据列表
}
