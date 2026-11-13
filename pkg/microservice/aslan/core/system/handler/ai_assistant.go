package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

var checkAIAssistantEnterpriseLicense = commonutil.CheckZadigEnterpriseLicense
var newAIAssistantContext = internalhandler.NewContextWithAuthorization

func requireAIAssistantEnterprise(c *gin.Context) bool {
	if err := checkAIAssistantEnterpriseLicense(); err != nil {
		var licenseError *e.HTTPError
		if errors.As(err, &licenseError) && licenseError.Code() == e.ErrLicenseInvalid.Code() {
			c.AbortWithStatusJSON(e.ErrorMessage(e.ErrLicenseInvalid))
		} else {
			c.AbortWithStatusJSON(e.ErrorMessage(e.NewWithDesc(e.ErrInternalError, "企业版许可证校验失败，请稍后重试")))
		}
		return false
	}
	return true
}

// GetAIAssistantConfig godoc
// @Summary 获取 AI 助手配置
// @Description 仅有效企业版且具备 AI 能力查看或编辑权限的用户可读取 AI 功能开关、模型集成标识及聊天保留天数；不返回模型凭据。网关访问路径为 /api/aslan/system/ai/assistant。
// @Description 标准错误响应：配置读取/更新失败 code=7210/7211，参数错误 code=400，许可证无效 code=7050；业务错误使用 HTTP 400。
// @Tags system
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Success 200 {object} types.AIAssistantConfig "助手配置"
// @Failure 403 {object} e.ErrorResponse "缺少 AI 能力权限或登录态无效"
// @Failure 400 {object} e.ErrorResponse "许可证无效 code=7050 或配置读取失败 code=7210"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/system/ai/assistant [get]
func GetAIAssistantConfig(c *gin.Context) {
	ctx, err := newAIAssistantContext(c)
	if err != nil || !hasAIManagementPermission(ctx, false) {
		c.AbortWithStatusJSON(e.ErrorMessage(e.ErrForbidden))
		return
	}
	if !requireAIAssistantEnterprise(c) {
		return
	}

	defer internalhandler.JSONResponse(c, ctx)
	ctx.Resp, ctx.RespErr = service.GetAIAssistantConfig(ctx)
}

// UpdateAIAssistantConfig godoc
// @Summary 更新 AI 助手配置
// @Description 仅有效企业版且具备 AI 能力编辑权限的用户可写。enabled 控制 AI 功能，默认关闭；业务写入仍须用户确认并校验实际资源权限。关闭 AI 保留历史查询和取消能力。模型须选择已有集成，聊天保留天数为 30～365 天。网关访问路径为 /api/aslan/system/ai/assistant。
// @Description 标准错误响应：配置读取/更新失败 code=7210/7211，参数错误 code=400，许可证无效 code=7050；业务错误使用 HTTP 400。
// @Tags system
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param body body types.AIAssistantConfig true "完整助手配置"
// @Success 200 {object} types.AIAssistantConfig "保存后的配置"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050 或配置保存失败 code=7211"
// @Failure 403 {object} e.ErrorResponse "缺少 AI 能力权限或登录态无效"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/system/ai/assistant [put]
func UpdateAIAssistantConfig(c *gin.Context) {
	ctx, err := newAIAssistantContext(c)
	if err != nil || !hasAIManagementPermission(ctx, true) {
		c.AbortWithStatusJSON(e.ErrorMessage(e.ErrForbidden))
		return
	}
	if !requireAIAssistantEnterprise(c) {
		return
	}

	var args types.AIAssistantConfig
	if err := c.ShouldBindJSON(&args); err != nil {
		c.AbortWithStatusJSON(e.ErrorMessage(e.NewWithDesc(e.ErrInvalidParam, "AI 助手配置格式不正确")))
		return
	}

	if err := service.UpdateAIAssistantConfig(ctx, &args); err != nil {
		c.AbortWithStatusJSON(e.ErrorMessage(err))
		return
	}

	c.JSON(http.StatusOK, args)
}
