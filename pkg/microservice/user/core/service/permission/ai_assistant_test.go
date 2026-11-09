package permission

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
)

func TestAIAssistantReportPermissionMapsToUserSDK(t *testing.T) {
	actions := generateDefaultSystemActions()
	if actions.DataCenter.ViewAIAssistantReport {
		t.Fatal("普通用户不能默认获得助手报告权限")
	}
	modifySystemAction(actions, VerbGetDataCenterAIAssistantReport)
	raw, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var received userclient.SystemActions
	if err := json.Unmarshal(raw, &received); err != nil || received.DataCenter == nil || !received.DataCenter.ViewAIAssistantReport || received.DataCenter.ViewInsight || received.DataCenter.ViewAIReviewInsight {
		t.Fatalf("专用权限应独立授权并通过 user SDK 传递: %s %v", raw, err)
	}
	for _, path := range []string{"../../init/action_initialization.sql", "../../init/dm_action_initialization.sql"} {
		raw, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(raw), "查看 AI 助手报告") || !strings.Contains(string(raw), VerbGetDataCenterAIAssistantReport) {
			t.Fatalf("初始化 SQL 必须注册该权限: %s %v", path, err)
		}
	}
}
