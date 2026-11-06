package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	_ "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models" // Swagger 响应类型。
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util/ginzap"
)

func authorizeAIAssistantStats(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil || !isAIAssistantStatsAdmin(ctx) {
		c.AbortWithStatusJSON(e.ErrorMessage(e.ErrForbidden))
		return
	}
	if err := commonutil.CheckZadigEnterpriseLicense(); err != nil {
		var licenseError *e.HTTPError
		if errors.As(err, &licenseError) && licenseError.Code() == e.ErrLicenseInvalid.Code() {
			c.AbortWithStatusJSON(e.ErrorMessage(e.ErrLicenseInvalid))
		} else {
			c.AbortWithStatusJSON(e.ErrorMessage(e.NewWithDesc(e.ErrInternalError, "企业版许可证校验失败，请稍后重试")))
		}
		return
	}
}

func isAIAssistantStatsAdmin(ctx *internalhandler.Context) bool {
	return ctx != nil && ctx.UserID != "" && ctx.Resources != nil && ctx.Resources.IsSystemAdmin
}

// GetAIAssistantStatsOverview godoc
// @Summary 查询 AI 助手概览
// @Description 仅有效企业版的当前系统管理员可查。直接读取 MySQL 日聚合，不返回聊天正文；默认最近三十天，范围限最近三百六十五天且不能包含未来日期。返回整体指标、按模型 Token 用量、估算节省分钟及人天、当前系统用户总数 total_users。previous 返回紧邻且自然日数相同的上期（含 from、to 和指标）；上期超出三百六十五天保留范围时为 null。无反馈时采纳率和估算人天为 null。
// @Tags stat
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Param from query string false "开始日期，包含当日，格式 YYYY-MM-DD"
// @Param to query string false "结束日期，包含当日，格式 YYYY-MM-DD"
// @Success 200 {object} models.AIAssistantStatsOverviewResponse
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或统计失败 code=7212"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员或登录态无效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/aslan/stat/v2/ai_assistant/overview [get]
func GetAIAssistantStatsOverview(c *gin.Context) {
	result, err := service.GetAIAssistantStatsOverview(c.Request.Context(), c.Query("from"), c.Query("to"))
	respondAIAssistantStats(c, result, err)
}

// GetAIAssistantStatsTrends godoc
// @Summary 查询 AI 助手每日趋势
// @Description 仅有效企业版的当前系统管理员可查。直接读取 MySQL 日聚合，不返回聊天正文；默认最近三十天，范围限最近三百六十五天且不能包含未来日期。按日期升序返回有聚合记录的日期，维度字段为 date，不补齐无记录日期；每项含按模型 Token 用量 model_usage，不返回上期。
// @Tags stat
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Param from query string false "开始日期，包含当日，格式 YYYY-MM-DD"
// @Param to query string false "结束日期，包含当日，格式 YYYY-MM-DD"
// @Success 200 {object} models.AIAssistantStatsTrendsResponse
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或统计失败 code=7212"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员或登录态无效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/aslan/stat/v2/ai_assistant/trends [get]
func GetAIAssistantStatsTrends(c *gin.Context) {
	result, err := service.GetAIAssistantStatsTrends(c.Request.Context(), c.Query("from"), c.Query("to"))
	respondAIAssistantStats(c, result, err)
}

// GetAIAssistantStatsSkills godoc
// @Summary 查询 AI 助手 Skill 分类表现
// @Description 分类通过 ai_usage_daily 关联 ai_skill 获取，停用 Skill 不影响历史统计。
// @Description 仅有效企业版的当前系统管理员可查。直接读取 MySQL 日聚合，不返回聊天正文；默认最近三十天，范围限最近三百六十五天且不能包含未来日期。按 category 汇总并固定返回发布辅助、故障排查、实施辅助、巡检诊断四类，无数据的分类返回零计数与空比例。工作流和服务报错分析归入故障排查，项目和服务初始化归入实施辅助；活跃用户在分类内去重，比例按分类总量计算，不包含普通聊天及未分类 Skill。每类含按模型 Token 用量 model_usage，不返回上期。
// @Tags stat
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Param from query string false "开始日期，包含当日，格式 YYYY-MM-DD"
// @Param to query string false "结束日期，包含当日，格式 YYYY-MM-DD"
// @Success 200 {object} models.AIAssistantStatsSkillsResponse
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或统计失败 code=7212"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员或登录态无效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/aslan/stat/v2/ai_assistant/skills [get]
func GetAIAssistantStatsSkills(c *gin.Context) {
	result, err := service.GetAIAssistantStatsSkills(c.Request.Context(), c.Query("from"), c.Query("to"))
	respondAIAssistantStats(c, result, err)
}

// GetAIAssistantStatsRankings godoc
// @Summary 查询 AI 助手用户排行
// @Description 仅有效企业版的当前系统管理员可查。直接读取 MySQL 日聚合，不返回聊天正文；默认最近三十天，范围限最近三百六十五天且不能包含未来日期。最多二十条，按完成任务数、运行数降序，用户 ID 升序；返回 user_id 和当前用户名 user_name，已删除用户回退为 ID；每项含按模型 Token 用量 model_usage，不返回上期。
// @Tags stat
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Param from query string false "开始日期，包含当日，格式 YYYY-MM-DD"
// @Param to query string false "结束日期，包含当日，格式 YYYY-MM-DD"
// @Success 200 {object} models.AIAssistantStatsRankingsResponse
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或统计失败 code=7212"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员或登录态无效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/aslan/stat/v2/ai_assistant/rankings [get]
func GetAIAssistantStatsRankings(c *gin.Context) {
	result, err := service.GetAIAssistantStatsRankings(c.Request.Context(), c.Query("from"), c.Query("to"))
	respondAIAssistantStats(c, result, err)
}

func respondAIAssistantStats(c *gin.Context, result any, err error) {
	if err != nil {
		responseErr := e.NewWithDesc(e.ErrGetAIAssistantStats, "统计查询失败，请稍后重试")
		if errors.Is(err, service.ErrAIAssistantStatsRange) {
			responseErr = e.NewWithDesc(e.ErrInvalidParam, err.Error())
		} else {
			ginzap.WithContext(c).Sugar().Errorf("查询 AI 助手统计失败: %v", err)
			if errors.Is(err, service.ErrAIAssistantStatsUnavailable) {
				responseErr = e.NewWithDesc(e.ErrGetAIAssistantStats, service.ErrAIAssistantStatsUnavailable.Error())
			}
		}
		c.AbortWithStatusJSON(e.ErrorMessage(responseErr))
		return
	}
	c.JSON(http.StatusOK, result)
}
