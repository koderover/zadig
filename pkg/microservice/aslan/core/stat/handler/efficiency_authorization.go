package handler

import internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"

func authorizeEfficiencyStats(ctx *internalhandler.Context) bool {
	return ctx != nil && ctx.Resources != nil && (ctx.Resources.IsSystemAdmin ||
		(ctx.Resources.SystemActions != nil && ctx.Resources.SystemActions.DataCenter != nil && ctx.Resources.SystemActions.DataCenter.ViewInsight))
}
