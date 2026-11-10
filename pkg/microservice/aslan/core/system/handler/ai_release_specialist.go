package handler

import (
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 获取发布专员模型配置
// @Description 需要 AI 能力查看或编辑权限；llm_integration_id 为空表示使用默认模型。模型选项使用 GET /api/aslan/system/llm/integrations，仅返回 ID 和名称。
// @Tags system
// @Produce json
// @Success 200 {object} commonmodels.AIReleaseSpecialistConfig
// @Router /api/aslan/system/ai/release-specialist [get]
func GetAIReleaseSpecialistConfig(c *gin.Context) {
	ctx, allowed := newAIManagementContext(c, false)
	if !allowed {
		return
	}
	defer internalhandler.JSONResponse(c, ctx)
	ctx.Resp, ctx.RespErr = service.GetAIReleaseSpecialistConfig(ctx)
}

// @Summary 保存发布专员模型配置
// @Description 需要 AI 能力编辑权限；选择已有模型，空 ID 恢复使用默认模型。已选模型失效时运行报错，不自动切换默认模型。
// @Tags system
// @Accept json
// @Produce json
// @Param body body commonmodels.AIReleaseSpecialistConfig true "发布专员模型配置"
// @Success 200
// @Router /api/aslan/system/ai/release-specialist [put]
func UpdateAIReleaseSpecialistConfig(c *gin.Context) {
	ctx, allowed := newAIManagementContext(c, true)
	if !allowed {
		return
	}
	defer internalhandler.JSONResponse(c, ctx)
	var args commonmodels.AIReleaseSpecialistConfig
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrInvalidParam, "发布专员配置格式不正确")
		return
	}
	ctx.RespErr = service.UpdateAIReleaseSpecialistConfig(ctx, &args)
}
