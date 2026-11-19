package handler

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestAIInsightsNavigationPermission(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  *internalhandler.Context
		want bool
	}{
		{name: "nil"},
		{name: "empty", ctx: &internalhandler.Context{}},
		{name: "resources", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{}}},
		{name: "actions", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{}}}},
		{name: "unrelated", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewInsight: true}}}}},
		{name: "review", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewAIReviewInsight: true}}}}, want: true},
		{name: "assistant", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewAIAssistantReport: true}}}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasNavigationPermission(tc.ctx, config.NavigationKeyAIInsights); got != tc.want {
				t.Fatalf("hasNavigationPermission = %v, want %v", got, tc.want)
			}
		})
	}
	if isEnterpriseLicenseRequiredKey(config.NavigationKeyAIInsights) {
		t.Fatal("AI 效能报告导航应使用 AI 功能授权检查")
	}
	if !isLicenseFeatureRequiredKey(config.NavigationKeyAIInsights) {
		t.Fatal("AI 效能报告导航应要求 AI 功能授权")
	}
}
