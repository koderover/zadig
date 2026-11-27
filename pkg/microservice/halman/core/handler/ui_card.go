package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// uiCards godoc
// @Summary 查询 AI 助手会话的卡片历史
// @Description 按源运行和卡片顺序倒序分页，仅返回当前用户有权查看的卡片。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "会话 ID"
// @Param cursor query string false "上一页返回的 next_cursor"
// @Param limit query int false "每页卡片数量" default(50) minimum(1) maximum(50)
// @Success 200 {object} object{items=[]models.UICard,next_cursor=string} "卡片列表和分页游标"
// @Failure 400 {object} errors.ErrorResponse "参数或分页游标无效，或 AI 助手业务错误"
// @Failure 401 {object} errors.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} errors.ErrorResponse "会话不存在、不属于本人或已删除"
// @Failure 500 {object} errors.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/conversations/{id}/cards [get]
func (a *API) uiCards(c *gin.Context) {
	limit, ok := page(c, 50)
	if !ok {
		return
	}
	if limit > 50 {
		respondError(c, fmt.Errorf("%w: limit 必须在 1 到 50 之间", common.ErrInvalidRequest))
		return
	}
	cards, next, err := a.Agent.Store.ListUICards(c.Request.Context(), requestUser(c), c.Param("id"), c.Query("cursor"), limit, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}
	cards, err = a.Agent.FilterUICards(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), cards, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": cards, "next_cursor": next})
}

// uiSubmit godoc
// @Summary 提交 AI 助手卡片并启动续跑
// @Description 提交待填写卡片，revision 必须匹配当前版本。values 按字段 ID 传值，资源选择项传 resource 对象或对象数组。
// @Description 同一 request_id 和相同内容复用已受理的提交。后续可通过 current_attempt_run_id 订阅运行。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "源运行 ID"
// @Param card_id path string true "卡片 ID"
// @Param body body models.UISubmissionRequest true "请求 ID、卡片修订号、动作 ID 和字段值"
// @Success 200 {object} models.UISubmissionResult "更新后的卡片和续跑信息"
// @Failure 400 {object} errors.ErrorResponse "参数无效，或卡片状态、修订号及请求 ID 冲突（code=7216）"
// @Failure 401 {object} errors.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} errors.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} errors.ErrorResponse "运行或卡片不存在、已到期或不可见"
// @Failure 500 {object} errors.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/cards/{card_id}/submissions [post]
func (a *API) uiSubmit(c *gin.Context) {
	var req models.UISubmissionRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if c.ShouldBindJSON(&req) != nil {
		respondError(c, common.ErrInvalidRequest)
		return
	}
	result, err := a.Agent.SubmitUICard(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), c.Param("card_id"), req, requestConfig(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// uiRetry godoc
// @Summary 重试已中断的 AI 助手卡片续跑
// @Description 当前尝试为 interrupted 且任务未被接管时，可沿用已保存的提交值启动新尝试。新尝试使用新的 request_id，同一次重试因超时重发时复用原 request_id。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "源运行 ID"
// @Param card_id path string true "卡片 ID"
// @Param submission_id path string true "提交 ID"
// @Param body body RequestIDRequest true "重试请求 ID"
// @Success 200 {object} models.UISubmissionResult "更新后的卡片和续跑信息"
// @Failure 400 {object} errors.ErrorResponse "参数无效，或提交、续跑状态及请求 ID 冲突（code=7216）"
// @Failure 401 {object} errors.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} errors.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} errors.ErrorResponse "运行或卡片不存在、已到期或不可见"
// @Failure 500 {object} errors.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/cards/{card_id}/submissions/{submission_id}/retry [post]
func (a *API) uiRetry(c *gin.Context) {
	var req RequestIDRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if c.ShouldBindJSON(&req) != nil {
		respondError(c, common.ErrInvalidRequest)
		return
	}
	result, err := a.Agent.RetryUISubmission(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), c.Param("card_id"), c.Param("submission_id"), req.RequestID, requestConfig(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// uiOptions godoc
// @Summary 查询 AI 助手资源选择卡的候选项
// @Description 按当前用户权限查询项目、环境或服务。级联查询通过 parent_values 传入父字段选中的 resource 对象。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "源运行 ID"
// @Param card_id path string true "卡片 ID"
// @Param field_id query string true "资源字段 ID"
// @Param query query string false "候选项搜索关键字"
// @Param cursor query string false "上一页返回的 next_cursor"
// @Param parent_values query string false "以父字段 ID 为键、resource 对象为值的 JSON 字符串"
// @Success 200 {object} object{items=[]models.UIResourceOption,next_cursor=string} "候选项和分页游标"
// @Failure 400 {object} errors.ErrorResponse "参数、父级资源或游标无效，或卡片状态冲突（code=7216）"
// @Failure 401 {object} errors.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} errors.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} errors.ErrorResponse "运行或卡片不存在、已到期或不可见"
// @Failure 500 {object} errors.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/cards/{card_id}/options [get]
func (a *API) uiOptions(c *gin.Context) {
	parents := map[string]any{}
	raw := c.Query("parent_values")
	if len(raw) > 16000 || (raw != "" && json.Unmarshal([]byte(raw), &parents) != nil) {
		respondError(c, common.ErrInvalidRequest)
		return
	}
	items, next, err := a.Agent.UIResourceOptions(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), c.Param("card_id"), c.Query("field_id"), c.Query("query"), c.Query("cursor"), parents, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": next})
}
