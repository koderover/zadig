package handler

import (
	"github.com/gin-gonic/gin"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

var newAIManagementAuthorization = internalhandler.NewContextWithAuthorization

func hasAIManagementPermission(ctx *internalhandler.Context, edit bool) bool {
	if ctx == nil || ctx.UserID == "" || ctx.Resources == nil {
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	if ctx.Resources.SystemActions == nil || ctx.Resources.SystemActions.AIManagement == nil {
		return false
	}
	actions := ctx.Resources.SystemActions.AIManagement
	return actions.Edit || (!edit && actions.View)
}

func newAIManagementContext(c *gin.Context, edit bool) (*internalhandler.Context, bool) {
	ctx, err := newAIManagementAuthorization(c)
	if err != nil || !hasAIManagementPermission(ctx, edit) {
		c.AbortWithStatusJSON(e.ErrorMessage(e.ErrForbidden))
		return ctx, false
	}
	return ctx, true
}
