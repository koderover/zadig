package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestGetWorkflowTaskListSettingsRequiresAuthorization(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	for _, token := range []string{"", "invalid-token"} {
		t.Run(token, func(t *testing.T) {
			resp := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(resp)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/system/workflowTaskList", nil)
			ctx.Request.Header.Set(setting.AuthorizationHeader, token)

			GetWorkflowTaskListSettings(ctx)

			if resp.Code != http.StatusForbidden {
				t.Fatalf("status = %d; want %d", resp.Code, http.StatusForbidden)
			}
		})
	}
}
