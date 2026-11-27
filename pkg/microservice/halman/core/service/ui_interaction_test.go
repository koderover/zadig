package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestUIOptionsApplyParentAndCurrentPermissions(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Hour)
	expires := now.Add(time.Hour)
	card := models.UICard{
		SchemaVersion: 1, ID: "card", RunID: "run", ConversationID: "conversation", Type: "resource_selector", Title: "选择环境", ExpiresAt: &expires,
		Data: common.JSON(models.UIData{Fields: []models.UIField{
			{ID: "project", Label: "项目", ResourceType: "project", Required: true},
			{ID: "env", Label: "环境", ResourceType: "environment", ParentFieldID: "project", Required: true},
		}}),
	}
	expectCard := func() {
		mock.ExpectQuery("SELECT c.payload,c.status,c.revision").WithArgs("user", "user", cutoff, cutoff, "run", 51).WillReturnRows(
			sqlmock.NewRows([]string{"payload", "status", "revision", "draft_fields", "run_status", "submission_id", "action_id", "first_run", "attempt_run", "attempt_status", "selected", "latest"}).
				AddRow(string(common.JSON(card)), "pending", 2, false, "awaiting_input", nil, nil, nil, nil, "", nil, "run"))
		mock.ExpectQuery("SELECT id,context,resource_scopes FROM ai_run").WithArgs("user", "conversation", cutoff, "run").
			WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", "[]"))
	}
	allowed := true
	environmentReads := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			if allowed {
				w.Write([]byte(`{"project_auth_info":{"demo":{"is_system_admin":true}}}`))
			} else {
				w.Write([]byte(`{}`))
			}
		case "/api/environment/environments":
			environmentReads++
			if r.URL.Query().Get("projectName") != "demo" {
				t.Error("child query did not use the selected project")
			}
			w.Write([]byte(`[{"env_name":"dev"},{"env_name":"qa"}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	service := &AgentService{Store: store, Platform: platform}
	ctx := context.Background()
	expectCard()
	options, _, err := service.UIResourceOptions(ctx, "user", token("user"), "run", "card", "env", "dev", "", nil, cutoff)
	if err != nil || len(options) != 0 || environmentReads != 0 {
		t.Fatalf("query without parent returned %v or called upstream: %v", options, err)
	}

	parents := map[string]any{"project": models.UIResource{ProjectKey: "demo"}}
	expectCard()
	options, _, err = service.UIResourceOptions(ctx, "user", token("user"), "run", "card", "env", "dev", "", parents, cutoff)
	if err != nil || len(options) != 1 || options[0].Resource.EnvName != "dev" || options[0].Resource.ProjectKey != "demo" {
		t.Fatalf("wrong scoped options: %v %v", options, err)
	}
	allowed = false
	expectCard()
	_, _, err = service.UIResourceOptions(ctx, "user", token("user"), "run", "card", "env", "", "", parents, cutoff)
	var denied *PlatformError
	if !errors.As(err, &denied) || denied.Status != http.StatusForbidden || environmentReads != 1 {
		t.Fatalf("revoked project had access: %v", err)
	}
}

func TestCardTextReplyRecoversChildDraftUsingRootStatus(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Hour)
	plan := string(common.JSON(models.OperationPlan{Draft: &models.OnboardingInput{ProjectKey: "demo"}}))
	mock.ExpectQuery("SELECT r.id, r.request_id,").WithArgs("child", "user", "user", cutoff).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "conversation_id", "status", "context", "created_at", "finished_at", "error",
			"input_tokens", "output_tokens", "skill_id", "skill_execution_id", "skill_status", "plan",
			"plan_version", "plan_status", "plan_expires_at", "answer_version", "question_source_run_id", "answer_available",
		}).AddRow("child", "request", "conversation", "awaiting_input", "{}", now, now, "", 0, 0, "project_onboarding", "root", nil, plan, 0, "awaiting_input", nil, 1, nil, false))
	mock.ExpectQuery("SELECT r.id,r.conversation_id,r.created_at").WithArgs("root", "user", "user", cutoff).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}).
			AddRow("root", "conversation", now, "project_onboarding", "root", "awaiting_input"))
	service := &AgentService{Store: store}
	source, err := service.readCardReplySource(context.Background(), &execution{
		User: "user", Cutoff: cutoff, Run: &models.Run{ConversationID: "conversation", RepliedCardRunID: "child"},
	})
	if err != nil || source.ID != "child" || source.SkillStatus == nil || *source.SkillStatus != "awaiting_input" {
		t.Fatalf("text reply could not continue the child draft: %#v, %v", source, err)
	}
}
