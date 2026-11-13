package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// retry godoc
// @Summary 为 AI 助手失败项生成重试清单
// @Description 仅接受仍有效且没有未知结果的当前失败计划，保留成功项及其结果。重新预检权限、模板、资源冲突和实际影响后，返回新的待确认运行，共用原 Skill 根执行；本接口不调用模型或提交业务写入，必须再次确认后才执行。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "当前失败计划的运行 ID"
// @Param body body RequestIDRequest true "新请求标识，重复请求使用同一标识"
// @Success 201 {object} models.Run "新建或幂等返回的待确认运行；聊天记录保存失败时 error 字段会给出提示"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} e.ErrorResponse "原计划或根执行不可用"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/retry [post]
func (a *API) retry(c *gin.Context) {
	var req RequestIDRequest
	if c.ShouldBindJSON(&req) != nil {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "需要有效的 request_id"))
		return
	}

	run, err := a.Agent.Retry(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), req.RequestID, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, run)
}

// reconcile godoc
// @Summary 只读核实 AI 助手未知操作结果
// @Description 重新检查当前权限，只读查询实际资源或发布记录并保存核实结果，不提交或重放业务写请求。资源返回 404 或证据不足时仍保持 unknown；已知状态直接返回快照。版本、原快照及根执行的当前计划指针在写回前再次检查。
// @Description 错误使用标准响应；业务错误通过 HTTP 400 返回，code=7050 或 7213～7221，具体原因见 message/description。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "待核实计划的运行 ID"
// @Param body body ReconcileRequest true "需要核实的计划版本"
// @Success 200 {object} models.Run "核实后的运行快照，状态可能仍为 unknown"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或 AI 助手业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "运行、根执行或必要的权限范围不可用"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Failure 403 {object} e.ErrorResponse "当前用户没有相应资源权限"
// @Router /api/halman/runs/{id}/reconcile [post]
func (a *API) reconcile(c *gin.Context) {
	var req ReconcileRequest
	if c.ShouldBindJSON(&req) != nil || req.Version < 1 {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "必须提供需要核实的清单版本"))
		return
	}

	run, err := a.Agent.Reconcile(c.Request.Context(), requestUser(c), c.GetHeader("Authorization"), c.Param("id"), req.Version, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, run)
}
