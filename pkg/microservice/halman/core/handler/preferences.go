package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// getPreferences godoc
// @Summary 获取本人 AI 助手偏好
// @Tags AI助手
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Success 200 {object} models.ModelSelection
// @Router /api/halman/preferences [get]
func (a *API) getPreferences(c *gin.Context) {
	preference, err := a.Agent.Store.ModelPreference(c.Request.Context(), requestUser(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preference)
}

// putPreferences godoc
// @Summary 保存本人 AI 助手偏好
// @Description 请求体传空对象 {} 时清除本人偏好。
// @Tags AI助手
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param body body models.ModelSelection true "模型和思考强度"
// @Success 200 {object} models.ModelSelection
// @Failure 403 {object} e.ErrorResponse "管理员未开放个人模型偏好"
// @Router /api/halman/preferences [put]
func (a *API) putPreferences(c *gin.Context) {
	cfg := requestConfig(c)
	if !cfg.AllowUserModelSelection {
		respondError(c, e.ErrForbidden)
		return
	}
	var preference *models.ModelSelection
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&preference) != nil || preference == nil || decoder.Decode(new(any)) != io.EOF {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "AI 助手偏好格式不正确"))
		return
	}
	if err := a.Agent.UpdateModelPreference(c.Request.Context(), requestUser(c), cfg, *preference); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preference)
}
