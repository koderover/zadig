package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func (a *API) requireSkillAdmin(c *gin.Context) {
	if a.Agent.Platform.Users == nil {
		respondError(c, e.NewWithDesc(e.ErrInternalError, "用户权限服务未配置"))
		return
	}
	resources, err := a.Agent.Platform.Users.GetUserAuthInfo(requestUser(c))
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrInternalError, "读取用户权限失败，请稍后重试"))
		return
	}
	if resources == nil || !resources.IsSystemAdmin {
		respondError(c, e.ErrForbidden)
	}
}

// listSkills godoc
// @Summary 查询 Skill
// @Description 仅有效企业版的系统管理员可查，AI 总开关关闭时仍可使用。返回 MySQL 中的完整中文定义、启停状态和追加规则，不包含默认提问。按四类分类、ID 排序。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Success 200 {object} object{items=[]models.Skill} "Skill 列表"
// @Failure 400 {object} e.ErrorResponse "许可证无效 code=7050 或读取失败 code=7219"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员"
// @Failure 500 {object} e.ErrorResponse "许可证或用户权限查询失败"
// @Router /api/halman/skills [get]
func (a *API) listSkills(c *gin.Context) {
	items, err := a.Agent.Store.ListSkills(c.Request.Context())
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, "读取 Skill 配置失败，请稍后重试"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// updateSkill godoc
// @Summary 更新 Skill 配置
// @Description 仅有效企业版的系统管理员可写，AI 总开关关闭时仍可使用。只能提交 enabled、custom_rules，两项均必填；追加规则最多 8000 字节 UTF-8 文本。恢复默认提交 enabled=true、custom_rules=""。规则影响后续生成，旧清单仍可确认；停用仅禁止新任务，既有任务可继续。不开放修改 Skill 正文、新增或删除 Skill。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 系统管理员登录令牌"
// @Param skill_id path string true "Skill ID"
// @Param body body models.SkillUpdate true "启停状态和追加规则"
// @Success 200 {object} models.Skill "保存后的 Skill"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或保存失败 code=7219"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户不是系统管理员"
// @Failure 404 {object} e.ErrorResponse "Skill 不存在"
// @Failure 500 {object} e.ErrorResponse "许可证或用户权限查询失败"
// @Router /api/halman/skills/{skill_id} [put]
func (a *API) updateSkill(c *gin.Context) {
	var req models.SkillUpdate
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil || !utf8.Valid(raw) {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "请求体须为有效 UTF-8 JSON"))
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || req.Enabled == nil || req.CustomRules == nil || !utf8.ValidString(*req.CustomRules) || len(*req.CustomRules) > 8000 {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "仅允许提交 enabled 和 custom_rules，规则须为最多 8000 字节的 UTF-8 文本"))
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "请求体必须是单个 JSON 对象"))
		return
	}
	item, err := a.Agent.Store.UpdateSkill(c.Request.Context(), c.Param("skill_id"), *req.Enabled, *req.CustomRules)
	if errors.Is(err, common.ErrNotFound) {
		respondError(c, e.NewWithDesc(e.ErrNotFound, "Skill 不存在"))
		return
	}
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, "保存 Skill 配置失败，请稍后重试"))
		return
	}
	c.JSON(http.StatusOK, item)
}
