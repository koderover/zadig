package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type API struct{ Agent *service.AgentService }

func (a *API) authenticate(c *gin.Context) {
	limit := int64(64 << 10)
	if c.FullPath() == "/api/halman/conversations/:id/runs" {
		// 标准客户端会回传历史和状态，单轮文本仍限制为 16000 字节。
		limit = 1 << 20
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	user, err := a.Agent.Platform.Authenticate(c.Request.Context(), c.GetHeader("Authorization"))
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrUnauthorized, "用户登录态无效或已失效"))
		return
	}

	c.Set("halman.user", user)
	if a.Agent.Platform.CheckEnterpriseLicense == nil {
		respondError(c, e.NewWithDesc(e.ErrInternalError, "企业版许可证校验未配置"))
		return
	}
	if err := a.Agent.Platform.CheckEnterpriseLicense(); err != nil {
		var licenseError *e.HTTPError
		if errors.As(err, &licenseError) && licenseError.Code() == e.ErrLicenseInvalid.Code() {
			respondError(c, e.ErrLicenseInvalid)
		} else {
			respondError(c, e.NewWithDesc(e.ErrInternalError, "企业版许可证校验失败，请稍后重试"))
		}
		return
	}
	cfg, err := a.Agent.Platform.Runtime(c.Request.Context())
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, "运行配置读取失败，请联系管理员"))
		return
	}

	c.Set("halman.config", cfg)
}

func requestConfig(c *gin.Context) *types.AIAssistantRuntime {
	return c.MustGet("halman.config").(*types.AIAssistantRuntime)
}

func requestUser(c *gin.Context) string { return c.GetString("halman.user") }

func (a *API) requireEnabled(c *gin.Context) {
	if !requestConfig(c).Enabled {
		respondError(c, common.ErrAIDisabled)
	}
}

func cutoff(c *gin.Context) time.Time {
	return time.Now().UTC().AddDate(0, 0, -requestConfig(c).HistoryRetentionDays)
}

func respondError(c *gin.Context, err error) {
	var httpError *e.HTTPError
	if errors.As(err, &httpError) {
		c.AbortWithStatusJSON(e.ErrorMessage(httpError))
		return
	}
	template, description := e.ErrAIAssistantRequest, "请稍后重试"
	var platform *service.PlatformError
	switch {
	case errors.Is(err, common.ErrAIDisabled):
		template, description = e.ErrAIAssistantDisabled, common.ErrAIDisabled.Error()
	case errors.As(err, &platform):
		template, description = e.ErrAIAssistantUpstream, platform.Error()
		if platform.Status == http.StatusUnauthorized || platform.Status == http.StatusForbidden {
			template = e.ErrForbidden
		}
		if platform.Uncertain {
			template = e.ErrAIAssistantResultUnknown
		}
	case errors.Is(err, common.ErrNotFound):
		template, description = e.ErrNotFound, common.ErrNotFound.Error()
	case errors.Is(err, common.ErrBusy):
		template, description = e.ErrAIAssistantBusy, common.ErrBusy.Error()
	case errors.Is(err, common.ErrRequestConflict):
		template, description = e.ErrAIAssistantConflict, common.ErrRequestConflict.Error()
	case errors.Is(err, common.ErrCapacity):
		template, description = e.ErrAIAssistantCapacity, common.ErrCapacity.Error()
	case errors.Is(err, common.ErrModelUnavailable):
		template, description = e.ErrAIAssistantModel, common.ErrModelUnavailable.Error()
	case errors.Is(err, common.ErrInvalidCursor), errors.Is(err, common.ErrInvalidRequest):
		template, description = e.ErrInvalidParam, err.Error()
	}
	c.AbortWithStatusJSON(e.ErrorMessage(e.NewWithDesc(template, description)))
}

func page(c *gin.Context, defaultLimit int) (int, bool) {
	limit := defaultLimit
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			respondError(c, e.NewWithDesc(e.ErrInvalidParam, "limit 必须在 1 到 100 之间"))
			return 0, false
		}

		limit = parsed
	}

	if _, err := common.DecodeCursor(c.Query("cursor")); err != nil {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, err.Error()))
		return 0, false
	}

	return limit, true
}

// createConversation godoc
// @Summary 新建 AI 助手会话
// @Description 创建当前登录用户的会话；会话不绑定项目，切换页面或项目无需新建会话。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Success 201 {object} models.Conversation "新建的会话"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Router /api/halman/conversations [post]
func (a *API) createConversation(c *gin.Context) {
	item, err := a.Agent.Store.CreateConversation(c.Request.Context(), requestUser(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

// listConversations godoc
// @Summary 查询本人 AI 助手会话
// @Description 按最近活动时间和 ID 倒序游标分页，排除已删除和已到期会话。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param cursor query string false "上一页返回的 next_cursor，首次查询不传"
// @Param limit query int false "每页数量" default(30) minimum(1) maximum(100)
// @Success 200 {object} object{items=[]models.Conversation,next_cursor=string} "会话列表；next_cursor 为空表示没有下一页"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/conversations [get]
func (a *API) listConversations(c *gin.Context) {
	limit, ok := page(c, 30)
	if !ok {
		return
	}

	items, next, err := a.Agent.Store.ListConversations(c.Request.Context(), requestUser(c), c.Query("cursor"), limit, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": next})
}

// deleteConversation godoc
// @Summary 删除本人 AI 助手会话
// @Description 立即隐藏会话并使等待确认的计划失效，后台分批清理数据；有活动运行或待核实写结果时拒绝删除，不删除 Zadig 业务资源。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "会话 ID"
// @Success 204 "删除成功，无响应体"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "会话不存在、不属于本人或已删除"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/conversations/{id} [delete]
func (a *API) deleteConversation(c *gin.Context) {
	if err := a.Agent.Store.DeleteConversation(c.Request.Context(), requestUser(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// messages godoc
// @Summary 查询 AI 助手历史消息
// @Description 只读取本人未到期的历史快照，不重新要求业务资源权限。默认保留原问题及最新可用回答，include_versions=true 可查看旧版本。过滤后可能出现空页，next_cursor 非空时仍需继续分页。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "会话 ID"
// @Param cursor query string false "上一页返回的 next_cursor"
// @Param limit query int false "每页原始消息事件数量" default(50) minimum(1) maximum(100)
// @Param include_versions query bool false "是否包含回答旧版本" default(false)
// @Success 200 {object} object{items=[]models.HistoryEvent,next_cursor=string} "按消息时间和行 ID 倒序排列的历史事件"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "会话不存在、不属于本人或已删除"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/conversations/{id}/messages [get]
func (a *API) messages(c *gin.Context) {
	limit, ok := page(c, 50)
	if !ok {
		return
	}

	includeVersions := false
	if raw := c.Query("include_versions"); raw != "" {
		var err error
		includeVersions, err = strconv.ParseBool(raw)
		if err != nil {
			respondError(c, e.NewWithDesc(e.ErrInvalidParam, "include_versions 必须是布尔值"))
			return
		}
	}

	items, next, err := a.Agent.Store.Messages(c.Request.Context(), requestUser(c), c.Param("id"), c.Query("cursor"), limit, cutoff(c), includeVersions)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": next})
}

// getRun godoc
// @Summary 查询 AI 助手运行快照
// @Description 查询本人未到期运行的状态、计划版本和逐项结果；仅查看保存的快照，不刷新业务资源或重新执行。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "运行 ID"
// @Success 200 {object} models.Run "运行快照"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "运行不存在、不属于本人、已删除或已到期"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Router /api/halman/runs/{id} [get]
func (a *API) getRun(c *gin.Context) {
	item, err := a.Agent.Store.GetRun(c.Request.Context(), requestUser(c), c.Param("id"), cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

// cancelRun godoc
// @Summary 取消 AI 助手运行
// @Description 仅停止 running/executing 运行，或放弃当前 Skill 最新的 awaiting_input 补问；pending_confirmation 必须通过 confirmation 的 reject 拒绝，直接取消返回 HTTP 400/code 7216。cancelling 和终态重复取消不修改状态。持久化取消后才停止后台处理，不撤回已受理的业务操作，已提交结果仍需核实。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "运行 ID"
// @Success 204 "取消请求已处理，无响应体"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "运行不存在、不属于本人、已删除或已到期"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Router /api/halman/runs/{id}/cancel [post]
func (a *API) cancelRun(c *gin.Context) {
	if _, err := a.Agent.Store.GetRunReference(c.Request.Context(), requestUser(c), c.Param("id"), cutoff(c)); err != nil {
		respondError(c, err)
		return
	}

	if err := a.Agent.Cancel(c.Request.Context(), requestUser(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// confirm godoc
// @Summary 确认或拒绝 AI 助手操作清单
// @Description decision 为空或 confirm 时确认，reject 时拒绝；两者均检查本人归属、根执行的当前计划指针及 version，已替换的旧卡片返回 HTTP 400/code 7216。确认还检查三十分钟有效期及当前资源权限，之后异步执行；拒绝还要求根业务仍在等待确认，仅关闭当前 pending_confirmation 清单，不中断正在运行或执行的任务，允许关闭已过期但尚未被替换的清单。同版本重复确认不重放写入；同版本重复拒绝不重复更新统计。执行要求 enabled 开启，关闭 AI 后仍可拒绝计划。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "生成清单的运行 ID"
// @Param body body ConfirmationRequest true "计划版本和用户决策"
// @Success 202 {object} models.Run "确认后的运行快照"
// @Success 204 "拒绝清单成功，无响应体"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} e.ErrorResponse "运行或根执行不可用"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/confirmation [post]
func (a *API) confirm(c *gin.Context) {
	var req ConfirmationRequest
	if c.ShouldBindJSON(&req) != nil || req.Version < 1 {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "必须提供确认卡片版本"))
		return
	}

	if req.Decision != "" && req.Decision != "confirm" && req.Decision != "reject" {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "decision 只能为 confirm 或 reject"))
		return
	}

	if req.Decision == "reject" {
		if _, err := a.Agent.Store.GetRunReference(c.Request.Context(), requestUser(c), c.Param("id"), cutoff(c)); err != nil {
			respondError(c, err)
			return
		}

		if err := a.Agent.Store.RejectPlan(c.Request.Context(), requestUser(c), c.Param("id"), req.Version); err != nil {
			respondError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
		return
	}

	if !requestConfig(c).Enabled {
		respondError(c, common.ErrAIDisabled)
		return
	}
	r, err := a.Agent.Confirm(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), req.Version, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, r)
}

// feedback godoc
// @Summary 评价 AI 助手回答
// @Description 对本人已生成的指定回答版本评价一次；重复提交相同反馈幂等成功，修改已有反馈返回冲突。helpful=false 表示无帮助，不能省略该字段。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "回答所在的运行 ID"
// @Param body body FeedbackRequest true "回答版本及是否有帮助"
// @Success 204 "反馈已保存，无响应体"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "回答不可用或不属于本人"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/feedback [post]
func (a *API) feedback(c *gin.Context) {
	var req FeedbackRequest
	if c.ShouldBindJSON(&req) != nil || req.Version < 1 || req.Helpful == nil {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "必须指定回答版本和反馈"))
		return
	}

	r, err := a.Agent.Store.GetRunReference(c.Request.Context(), requestUser(c), c.Param("id"), cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	if err := a.Agent.Store.Feedback(c.Request.Context(), requestUser(c), r, req.Version, *req.Helpful); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// recommendations godoc
// @Summary 获取 AI 助手推荐问题
// @Description 按页面候选上下文排列固定问题，每批最多三条；使用 next_batch 换一批。只返回推荐及候选上下文，不查询业务资源或直接执行任务。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param page_type query string false "当前页面类型，例如 workflow_task"
// @Param project_key query string false "页面候选项目标识"
// @Param workflow_name query string false "页面候选工作流标识"
// @Param task_id query int64 false "页面候选任务编号" minimum(0)
// @Param job_name query string false "页面候选步骤名称"
// @Param env_name query string false "页面候选环境名称"
// @Param service_name query string false "页面候选服务名称"
// @Param production query bool false "页面是否为生产环境" default(false)
// @Param batch query int false "轮换批次，首次查询使用 0，后续使用 next_batch" default(0) minimum(0) maximum(10000)
// @Success 200 {object} object{items=[]models.Recommendation,next_batch=integer} "推荐问题及下一批次"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/recommendations [get]
func (a *API) recommendations(c *gin.Context) {
	batch := 0
	if value := c.Query("batch"); value != "" {
		var err error
		batch, err = strconv.Atoi(value)
		if err != nil || batch < 0 || batch > 10000 {
			respondError(c, e.NewWithDesc(e.ErrInvalidParam, "batch 必须在 0 到 10000 之间"))
			return
		}
	}

	page := models.PageContext{
		PageType:     c.Query("page_type"),
		ProjectKey:   c.Query("project_key"),
		EnvName:      c.Query("env_name"),
		ServiceName:  c.Query("service_name"),
		WorkflowName: c.Query("workflow_name"),
		JobName:      c.Query("job_name"),
	}
	if value := c.Query("production"); value != "" {
		var err error
		page.Production, err = strconv.ParseBool(value)
		if err != nil {
			respondError(c, e.NewWithDesc(e.ErrInvalidParam, "production 必须是布尔值"))
			return
		}
	}

	if value := c.Query("task_id"); value != "" {
		var err error
		page.TaskID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || page.TaskID < 0 {
			respondError(c, e.NewWithDesc(e.ErrInvalidParam, "task_id 无效"))
			return
		}
	}

	skills, err := a.Agent.Store.ListSkills(c.Request.Context())
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, "读取 Skill 配置失败，请稍后重试"))
		return
	}
	items, next := service.Recommendations(page, batch, skills)
	c.JSON(http.StatusOK, gin.H{"items": items, "next_batch": next})
}

// regenerate godoc
// @Summary 重新生成 AI 助手最新回答
// @Description 使用原问题和原上下文快照重新回答，复查当前资源权限；不创建、修改或执行操作清单。原问题及 Skill 根执行必须仍在保留期内；生成失败保留原回答。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "需要替换的最新回答运行 ID"
// @Param body body RequestIDRequest true "新请求标识，重复提交同一请求时保持不变"
// @Success 202 {object} models.Run "重新回答的运行快照"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} e.ErrorResponse "原问题、回答或根执行已不可用"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/regenerate [post]
func (a *API) regenerate(c *gin.Context) {
	var req RequestIDRequest
	if c.ShouldBindJSON(&req) != nil || common.ValidateRequest(models.RunRequest{RequestID: req.RequestID, Message: "重新回答"}) != nil {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "需要有效的 request_id"))
		return
	}

	r, err := a.Agent.Regenerate(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), req.RequestID, requestConfig(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, r)
}
