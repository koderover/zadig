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
	"github.com/koderover/zadig/v2/pkg/types"
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

func TestResourceSubmissionLoadsEachFieldAndPermissionsOnce(t *testing.T) {
	counts := map[string]int{}
	service := &AgentService{Platform: testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		counts[r.URL.Path]++
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			w.Write([]byte(`{"project_auth_info":{"demo":{"is_system_admin":true}}}`))
		case "/api/v1/picket/projects":
			w.Write([]byte(`[{"project_name":"demo"}]`))
		case "/api/environment/environments":
			w.Write([]byte(`[{"env_name":"dev"},{"env_name":"qa"}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	u := models.UIContinuation{
		Card: models.UICard{
			SchemaVersion: 1, Type: "resource_selector", Title: "选择环境", Status: "pending", Revision: 2,
			Actions: []models.UIAction{{ID: "submit", Label: "提交", Type: "submit"}},
			Data: common.JSON(models.UIData{Fields: []models.UIField{
				{ID: "project", Label: "项目", ResourceType: "project", Required: true},
				{ID: "env", Label: "环境", ResourceType: "environment", Required: true, Multiple: true, ParentFieldID: "project"},
				{ID: "optional", Label: "其他环境", ResourceType: "environment", ParentFieldID: "project"},
			}}),
		},
		Revision: 2, ActionID: "submit", Values: map[string]any{
			"project": map[string]any{"project_key": "demo"},
			"env": []any{
				map[string]any{"project_key": "demo", "env_name": "dev"},
				map[string]any{"project_key": "demo", "env_name": "qa"},
			},
			"optional": nil,
		},
	}
	if err := service.prepareUICardRun(context.Background(), token("user"), &u); err != nil {
		t.Fatal(err)
	}
	if _, exists := u.Values["optional"]; exists || len(u.ResourceScopes) != 3 {
		t.Fatalf("expected only three actual choices, got values=%v scopes=%v", u.Values, u.ResourceScopes)
	}
	if len(counts) != 3 {
		t.Fatalf("unexpected upstream requests: %v", counts)
	}
	for path, calls := range counts {
		if calls != 1 {
			t.Errorf("%s called %d times, want 1", path, calls)
		}
	}
}

func TestLegacyEmptyResourceValuesKeepSubmittedCardVisible(t *testing.T) {
	store, mock := mockStore(t)
	cutoff := time.Now().Add(-time.Hour)
	mock.ExpectQuery("SELECT id,context,resource_scopes FROM ai_run").WithArgs("user", "conversation", cutoff, "run").
		WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", "[]"))
	service := &AgentService{Store: store}
	cards, err := service.FilterUICards(context.Background(), "user", "", []models.UICard{{
		ID: "card", RunID: "run", ConversationID: "conversation", Type: "resource_selector", Status: "submitted",
		Submission:         &models.UISubmissionResult{SubmissionID: "submission"},
		ResourceSelections: map[string]any{"unset": nil, "empty": "", "empty_list": []any{}},
	}}, cutoff)
	if err != nil || len(cards) != 1 || cards[0].Submission == nil {
		t.Fatalf("card hidden by empty selections: %v %v", cards, err)
	}
}

func TestCardSubmissionReplayOnlyLooksUpRequestOnce(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Hour)
	card := models.UICard{ID: "card", RunID: "source", ConversationID: "conversation-1", Title: "补参", Status: "submitted", Revision: 3}
	req := models.UISubmissionRequest{RequestID: "req-1", ActionID: "submit", Revision: 2, Values: map[string]any{}}
	fingerprint := common.RequestFingerprint(card.ConversationID, models.RunRequest{
		RequestID: req.RequestID, Message: card.Title,
		UIContinuation: &models.UIContinuation{Card: card, Hash: submissionHash(req)},
	})
	mock.ExpectQuery("SELECT r.id, r.request_id,").WithArgs("source", "user", "user", cutoff).WillReturnRows(runRows("source", now))
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WithArgs("user", "req-1", "user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}).AddRow("existing", fingerprint, now))
	mock.ExpectQuery("SELECT r.id, r.request_id,").WithArgs("existing", "user", "user", sqlmock.AnyArg()).WillReturnRows(runRows("existing", now))
	mock.ExpectQuery("SELECT c.payload,c.status,c.revision").WithArgs("user", "user", cutoff, cutoff, "source", 51).
		WillReturnRows(sqlmock.NewRows([]string{"payload", "status", "revision", "draft_fields", "run_status", "submission_id", "action_id", "first_run", "attempt_run", "attempt_status", "selected", "latest"}).
			AddRow(string(common.JSON(card)), "submitted", 3, false, "awaiting_input", "submission", "submit", "existing", "existing", "running", nil, "existing"))
	mock.ExpectQuery("SELECT id,context,resource_scopes FROM ai_run").WithArgs("user", "conversation-1", cutoff, "source").
		WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("source", "{}", "[]"))
	service := &AgentService{Store: store}
	cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{HistoryRetentionDays: 90}}
	result, err := service.startUICardRun(context.Background(), "user", "", card, req, "", cfg, cutoff)
	if err != nil || result == nil || result.CurrentAttemptRunID != "existing" {
		t.Fatalf("replay should return the committed run from a single lookup: %v, %v", result, err)
	}
}
