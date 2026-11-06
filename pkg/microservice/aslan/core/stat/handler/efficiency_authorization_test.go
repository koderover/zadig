package handler

import (
	"testing"

	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestEfficiencyAnalysisUsesSystemInsightPermission(t *testing.T) {
	for _, input := range []struct {
		name      string
		resources *userclient.AuthorizedResources
		allowed   bool
	}{
		{name: "权限缺失"},
		{name: "普通用户", resources: &userclient.AuthorizedResources{}},
		{name: "仅项目管理员", resources: &userclient.AuthorizedResources{ProjectAuthInfo: map[string]*userclient.ProjectActions{"p": {IsProjectAdmin: true}}}},
		{name: "仅代码审查洞察", resources: &userclient.AuthorizedResources{SystemActions: &userclient.SystemActions{DataCenter: &userclient.DataCenterActions{ViewAIReviewInsight: true}}}},
		{name: "有洞察无项目权限", resources: &userclient.AuthorizedResources{SystemActions: &userclient.SystemActions{DataCenter: &userclient.DataCenterActions{ViewInsight: true}}}, allowed: true},
		{name: "系统管理员", resources: &userclient.AuthorizedResources{IsSystemAdmin: true}, allowed: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			if authorizeEfficiencyStats(&internalhandler.Context{Resources: input.resources}) != input.allowed {
				t.Fatal("效能分析只按系统洞察权限授权")
			}
		})
	}
}
