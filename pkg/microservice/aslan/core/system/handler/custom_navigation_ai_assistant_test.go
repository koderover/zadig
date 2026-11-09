package handler

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestAIAssistantReportNavigationRequiresDedicatedPermission(t *testing.T) {
	for _, ctx := range []*internalhandler.Context{
		nil,
		{},
		{Resources: &user.AuthorizedResources{}},
		{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{}}},
		{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewInsight: true, ViewAIReviewInsight: true}}}},
	} {
		if hasNavigationPermission(ctx, config.NavigationKeyAIAssistantReport) {
			t.Fatal("缺少专用报告权限的普通用户不能看到入口")
		}
	}
	ctx := &internalhandler.Context{Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewAIAssistantReport: true}}}}
	if !hasNavigationPermission(ctx, config.NavigationKeyAIAssistantReport) || !isEnterpriseLicenseRequiredKey(config.NavigationKeyAIAssistantReport) {
		t.Fatal("助手报告导航应同时使用专用权限及企业版检查")
	}
}
