package handler

import (
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 获取审计分析模型配置
// @Description 需要 AI 能力查看或编辑权限；llm_integration_id 为空表示未配置模型。模型选项使用 GET /api/aslan/system/llm/integrations，仅返回 ID 和名称。
// @Tags system
// @Produce json
// @Success 200 {object} commonmodels.AIAuditConfig
// @Router /api/aslan/system/ai/audit [get]
func GetAIAuditConfig(c *gin.Context) {
	ctx, allowed := newAIManagementContext(c, false)
	if !allowed {
		return
	}
	defer internalhandler.JSONResponse(c, ctx)
	ctx.Resp, ctx.RespErr = service.GetAIAuditConfig(ctx)
}

// @Summary 保存审计分析模型配置
// @Description 需要 AI 能力编辑权限；必须选择已有模型。未配置或已选模型失效时审计分析报错。
// @Tags system
// @Accept json
// @Produce json
// @Param body body commonmodels.AIAuditConfig true "审计分析模型配置"
// @Success 200
// @Router /api/aslan/system/ai/audit [put]
func UpdateAIAuditConfig(c *gin.Context) {
	ctx, allowed := newAIManagementContext(c, true)
	if !allowed {
		return
	}
	defer internalhandler.JSONResponse(c, ctx)
	var args commonmodels.AIAuditConfig
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrInvalidParam, "审计分析配置格式不正确")
		return
	}
	ctx.RespErr = service.UpdateAIAuditConfig(ctx, &args)
}
