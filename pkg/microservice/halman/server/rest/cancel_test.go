package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func TestCancelAndRejectHaveSeparateHTTPBoundaries(t *testing.T) {
	for _, input := range []struct {
		name, path, body, active, status, planStatus string
		want                                         int
	}{
		{"取消不能代替拒绝", "cancel", "", "run", "pending_confirmation", "pending_confirmation", http.StatusBadRequest},
		{"拒绝当前清单", "confirmation", `{"version":1,"decision":"reject"}`, "run", "pending_confirmation", "pending_confirmation", http.StatusNoContent},
		{"旧卡片不能取消新清单", "confirmation", `{"version":1,"decision":"reject"}`, "new-run", "pending_confirmation", "superseded", http.StatusBadRequest},
		{"已确认执行的清单不能再拒绝", "confirmation", `{"version":1,"decision":"reject"}`, "run", "executing", "executing", http.StatusBadRequest},
		{"同版本重复拒绝幂等成功", "confirmation", `{"version":1,"decision":"reject"}`, "run", "cancelled", "cancelled", http.StatusNoContent},
	} {
		t.Run(input.name, func(t *testing.T) {
			r, mock := aguiTestEngine(t)
			mock.ExpectQuery("SELECT r.id,r.conversation_id,r.created_at").WithArgs("run", "user", "user", sqlmock.AnyArg()).
				WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}).AddRow("run", "conversation", time.Now().UTC(), "project_onboarding", "root", input.status))
			mock.ExpectQuery("SELECT conversation_id,skill_execution_id FROM ai_run").WithArgs("run", "user").
				WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "skill_execution_id"}).AddRow("conversation", "root"))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
			mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run .*FOR UPDATE").WithArgs("root", "user").
				WillReturnRows(sqlmock.NewRows([]string{"active_plan_run_id", "skill_status"}).AddRow(input.active, input.status))
			mock.ExpectQuery("SELECT status,plan_version,plan_status FROM ai_run .*FOR UPDATE").WithArgs("run", "user").
				WillReturnRows(sqlmock.NewRows([]string{"status", "plan_version", "plan_status"}).AddRow(input.status, 1, input.planStatus))
			if input.want == http.StatusNoContent {
				if input.status == "pending_confirmation" {
					mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution FROM ai_run .*FOR UPDATE").WithArgs("root", "user").
						WillReturnRows(sqlmock.NewRows([]string{"skill_id", "skill_status", "usage_contribution"}).AddRow("project_onboarding", "pending_confirmation", nil))
					mock.ExpectExec("INSERT INTO ai_usage_daily").WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectExec("UPDATE ai_run SET skill_status").WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectExec("UPDATE ai_run SET status='cancelled',plan_status='cancelled'").WithArgs(sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			req := httptest.NewRequest(http.MethodPost, "/api/halman/runs/run/"+input.path, strings.NewReader(input.body))
			req.Header.Set("Authorization", token("user"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != input.want {
				t.Fatalf("接口边界或错误契约不正确: %d %s", w.Code, w.Body)
			}
			if input.want == http.StatusBadRequest {
				assertAssistantErrorCode(t, w, e.ErrAIAssistantConflict.Code())
			}
		})
	}
}
