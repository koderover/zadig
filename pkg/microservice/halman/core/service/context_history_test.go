package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestSessionHistoryDoesNotRequireMatchingPage(t *testing.T) {
	for _, tc := range []struct {
		name        string
		page        models.PageContext
		scopes      string
		permissions string
		keep        bool
	}{
		{"service_to_environment", models.PageContext{ProjectKey: "p", EnvName: "dev"}, `[{"project_key":"p","env_name":"dev","service_name":"api"}]`, `{"project_auth_info":{"p":{"env":{"view":true}}}}`, true},
		{"different_environment", models.PageContext{ProjectKey: "p", EnvName: "other"}, `[{"project_key":"p","env_name":"dev","service_name":"api"}]`, `{"project_auth_info":{"p":{"env":{"view":true}}}}`, true},
		{"different_project", models.PageContext{ProjectKey: "other"}, `[{"project_key":"p","env_name":"dev","service_name":"api"}]`, `{"project_auth_info":{"p":{"env":{"view":true}}}}`, true},
		{"revoked_resource", models.PageContext{ProjectKey: "p", EnvName: "dev"}, `[{"project_key":"p","env_name":"dev","service_name":"api"}]`, `{}`, false},
		{"plain_chat_with_page", models.PageContext{ProjectKey: "p", EnvName: "dev"}, `[]`, "", true},
		{"catalog_stays_excluded", models.PageContext{ProjectKey: "p", EnvName: "dev"}, `[{"catalog":"projects"}]`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			now := time.Now().UTC()
			mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).
				AddRow("past", `{"project_key":"p","env_name":"dev","service_name":"api"}`, tc.scopes))
			mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{"id", "origin", "version", "regenerated", "question_available", "latest"}).
				AddRow("past", "past", 1, false, true, "past"))
			checks := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				checks++
				if r.URL.Path != "/api/v1/authorization/auth-info" || r.Header.Get("Authorization") != token("user") || tc.permissions == "" {
					t.Fatalf("历史权限只应按实际资源复查: %s", r.URL)
				}
				w.Write([]byte(tc.permissions))
			})
			state := session.NewSession("halman", "user", "conversation")
			state.Events = []event.Event{historyEvent("past", "user", "看下 api 日志", now)}
			if tc.scopes != "[]" {
				call := historyEvent("past", "assistant", "", now)
				call.Choices[0].Message.ToolCalls = []model.ToolCall{{ID: "call", Type: "function", Function: model.FunctionDefinitionParam{Name: "service_diagnosis", Arguments: []byte(`{}`)}}}
				result := historyEvent("past", "tool", "日志证据", now)
				result.Choices[0].Message.ToolID = "call"
				state.Events = append(state.Events, call, result)
			}
			state.Events = append(state.Events, historyEvent("past", "assistant", "诊断回答", now))
			want := len(state.Events)
			if !tc.keep {
				want = 0
			}
			e := &execution{Run: &models.Run{ID: "current", ConversationID: "conversation", Context: tc.page}, User: "user", Authorization: token("user"), Cutoff: now.Add(-time.Hour)}
			s := &AgentService{Store: store, Platform: platform}
			got, err := s.sessionContext(&session.GetSessionContext{Context: context.WithValue(context.Background(), executionKey{}, e)}, func() (*session.Session, error) { return state, nil })
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Events) != want {
				t.Fatalf("页面变化不应丢失已授权的完整对话: got=%d want=%d", len(got.Events), want)
			}
			if (len(e.HistoryRunIDs) == 1 && e.HistoryRunIDs[0] == "past") != tc.keep {
				t.Fatalf("仅保留的历史应进入每次模型调用前的权限复查: %v", e.HistoryRunIDs)
			}
			if (checks == 1) != (tc.permissions != "") {
				t.Fatalf("权限复查次数错误: %d", checks)
			}
		})
	}
}
