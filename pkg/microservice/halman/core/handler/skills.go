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
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func (a *API) requireSkillManagementPermission(c *gin.Context) {
	if a.Agent.Platform.Users == nil {
		respondError(c, e.NewWithDesc(e.ErrInternalError, "用户权限服务未配置"))
		return
	}
	resources, err := a.Agent.Platform.Users.GetUserAuthInfo(requestUser(c))
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrInternalError, "读取用户权限失败，请稍后重试"))
		return
	}
	if resources == nil {
		respondError(c, e.ErrForbidden)
		return
	}
	if resources.IsSystemAdmin {
		return
	}
	if resources.SystemActions != nil && resources.SystemActions.AIManagement != nil {
		actions := resources.SystemActions.AIManagement
		if actions.Edit || (c.Request.Method == http.MethodGet && actions.View) {
			return
		}
	}
	respondError(c, e.ErrForbidden)
}

// listSkills godoc
// @Summary 查询 Skill
// @Description 仅有效企业版且具有 AI 能力查看或编辑权限的用户可查，AI 总开关关闭时仍可使用。按系统语言返回内置 Skill 的名称、说明、正文和 category_name；category 保留原始分类值，追加规则保留管理员原文。数据库和运行时仍使用中文定义，未知定义保留原文。不包含默认提问，按原始分类、ID 排序。
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Success 200 {object} object{items=[]models.SkillView} "Skill 列表"
// @Failure 400 {object} e.ErrorResponse "许可证无效 code=7050 或读取失败 code=7219"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户缺少 AI 能力权限"
// @Failure 500 {object} e.ErrorResponse "许可证或用户权限查询失败"
// @Router /api/halman/skills [get]
func (a *API) listSkills(c *gin.Context) {
	items, err := a.Agent.Store.ListSkills(c.Request.Context())
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, responseLanguage(c).Text("读取 Skill 配置失败，请稍后重试", "Failed to load Skill configuration; please try again later")))
		return
	}
	views := make([]models.SkillView, 0, len(items))
	for _, item := range items {
		views = append(views, builtins.Display(item, responseLanguage(c)))
	}
	c.JSON(http.StatusOK, gin.H{"items": views})
}

// updateSkill godoc
// @Summary 更新 Skill 配置
// @Description 仅有效企业版且具有 AI 能力编辑权限的用户可写，AI 总开关关闭时仍可使用。只能提交 enabled、custom_rules，两项均必填；追加规则最多 8000 字节 UTF-8 文本。恢复默认提交 enabled=true、custom_rules=""。规则影响后续生成，旧清单仍可确认；停用仅禁止新任务，既有任务可继续。不开放修改 Skill 正文、新增或删除 Skill。
// @Description 保存后按当前系统语言返回展示定义和 category_name；category、追加规则及数据库中的原始定义不翻译。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param skill_id path string true "Skill ID"
// @Param body body models.SkillUpdate true "启停状态和追加规则"
// @Success 200 {object} models.SkillView "保存后的 Skill 展示信息"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或保存失败 code=7219"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户缺少 AI 能力权限"
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
		respondError(c, e.NewWithDesc(e.ErrAIAssistantRuntime, responseLanguage(c).Text("保存 Skill 配置失败，请稍后重试", "Failed to save Skill configuration; please try again later")))
		return
	}
	c.JSON(http.StatusOK, builtins.Display(*item, responseLanguage(c)))
}
