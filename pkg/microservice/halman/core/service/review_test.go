package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
	"github.com/koderover/zadig/v2/pkg/types"
)

func historyEvent(run, role, text string, at time.Time) event.Event {
	e := event.NewResponseEvent("inv", role, &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.Role(role), Content: text}}}})
	e.RequestID = run
	e.Timestamp = at
	return *e
}

func TestLatestAnswerContextAndRegeneration(t *testing.T) {
	for _, kind := range []string{"latest", "regenerate", "expired_question"} {
		t.Run(kind, func(t *testing.T) {
			store, mock := mockStore(t)
			now := time.Now().UTC()
			cutoff := now.Add(-time.Hour)
			state := session.NewSession("halman", "user", "conversation")
			state.Events = []event.Event{
				historyEvent("original", "user", "原问题", now),
				historyEvent("original", "assistant", "旧回答", now),
				historyEvent("new", "assistant", "新回答", now),
				historyEvent("failed", "assistant", "失败版本", now),
			}
			exec := &execution{
				Run:    &models.Run{ID: "current", ConversationID: "conversation"},
				User:   "user",
				Cutoff: cutoff,
			}
			if kind == "regenerate" {
				exec.RegenerateFrom = &models.Run{ID: "original"}
			}

			mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).
				AddRow("original", "{}", "[]").
				AddRow("new", "{}", "[]").
				AddRow("failed", "{}", "[]"))
			available := kind != "expired_question"
			mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{
				"id",
				"origin",
				"version",
				"regenerated",
				"question_available",
				"latest",
			}).
				AddRow("original", "original", 1, false, available, "new").
				AddRow("new", "original", 2, true, available, "new").
				AddRow("failed", "original", 3, true, available, "new"))
			s := &AgentService{Store: store}
			result, err := s.sessionContext(&session.GetSessionContext{Context: context.WithValue(context.Background(), executionKey{}, exec)}, func() (*session.Session, error) { return state, nil })
			if err != nil {
				t.Fatal(err)
			}

			if kind != "latest" {
				if len(result.Events) != 0 {
					t.Fatal("过期问题和正在重新回答的逻辑轮次不能进入上下文")
				}

				return
			}

			if len(result.Events) != 2 || result.Events[0].Choices[0].Message.Content != "原问题" ||
				result.Events[1].Choices[0].Message.Content != "新回答" {
				t.Fatalf("只应保留原问题和最新成功回答: %+v", result.Events)
			}
		})
	}
}

func TestLogicalTurnBudgetCannotKeepAnswerWithoutQuestion(t *testing.T) {
	now := time.Now()
	events := []event.Event{
		historyEvent("question", "user", "问题", now),
		historyEvent("answer", "assistant", "答案", now),
	}
	answer, _ := json.Marshal(events[1])
	selected := contextEvents(
		events,
		map[string]bool{"question": true, "answer": true},
		now.Add(-time.Hour),
		len(answer),
		map[string]string{"question": "turn", "answer": "turn"},
	)
	if len(selected) != 0 {
		t.Fatal("回答版本与原问题必须作为同一逻辑轮次裁剪")
	}
}

func TestEveryHistoricalResourceMustStillBeAuthorized(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now()
	cutoff := now.Add(-time.Hour)
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).
		AddRow("past", `{"project_key":"b"}`, `[{"project_key":"a"},{"project_key":"b"}]`))
	mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{
		"id",
		"origin",
		"version",
		"regenerated",
		"question_available",
		"latest",
	}).
		AddRow("past", "past", 1, false, true, "past"))
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/project/products/a" {
			t.Errorf("不应使用后读取的 B 覆盖 A 的权限范围: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusForbidden)
	})
	s := &AgentService{Store: store, Platform: platform}
	exec := &execution{
		Run: &models.Run{
			ID:             "current",
			ConversationID: "conversation",
			Context:        models.PageContext{ProjectKey: "b"},
		},
		User:          "user",
		Authorization: "user-token",
		Cutoff:        cutoff,
	}
	state := session.NewSession("halman", "user", "conversation")
	state.Events = []event.Event{historyEvent("past", "assistant", "来自 A 和 B 的结果", now)}
	result, err := s.sessionContext(&session.GetSessionContext{Context: context.WithValue(context.Background(), executionKey{}, exec)}, func() (*session.Session, error) { return state, nil })
	if err != nil || len(result.Events) != 0 {
		t.Fatalf("任一必要范围失权后都不能注入: %v", err)
	}
}

func TestRetryPlanPreservesSuccessAndRejectsUncertainty(t *testing.T) {
	original := models.OperationPlan{Items: []models.OperationItem{
		{ID: "done", Status: "succeeded", Result: common.JSON(map[string]bool{"kept": true})},
		{ID: "failed", Status: "failed", Error: "旧错误"},
		{ID: "blocked", Status: "blocked", DependsOn: []string{"failed"}},
	}}
	plan, err := retryPlan("source", original)
	if err != nil || plan.RetryOfRunID != "source" || plan.Items[0].Status != "succeeded" ||
		string(plan.Items[0].Result) != string(original.Items[0].Result) ||
		plan.Items[1].Status != "pending" ||
		plan.Items[2].Status != "pending" ||
		original.Items[1].Status != "failed" {
		t.Fatalf("重试不能重放或修改成功项: %+v %v", plan, err)
	}

	original.Items[1].Status = "unknown"
	if _, err := retryPlan("source", original); err == nil {
		t.Fatal("结果未核实不能生成重试清单")
	}

	if len(pendingPlan(plan).Items) != 2 {
		t.Fatal("预检只应包含需要执行的动作")
	}
}

func TestReconciliationDoesNotTreatAbsenceAsFailureOrResubmit(t *testing.T) {
	requests := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "user-token" {
			t.Error("核实只能透传用户身份做只读查询")
		}

		w.WriteHeader(http.StatusNotFound)
	})
	s := &AgentService{Platform: platform}
	exec := &execution{Run: &models.Run{CreatedAt: time.Now()}, Authorization: "user-token"}
	state, _, err := s.verifyOperationResult(
		context.Background(),
		exec,
		models.OperationItem{
			Action:     "create_project",
			ProjectKey: "project",
			Params:     common.JSON(models.ProjectInput{Key: "project"}),
		},
	)
	if err != nil || state != "unknown" || requests != 1 {
		t.Fatalf("缺少资源不能证明失败: %s %v", state, err)
	}
}

func TestRecommendationsUsePageAndRotateAtMostThree(t *testing.T) {
	for _, item := range []struct {
		page  models.PageContext
		first string
	}{
		{models.PageContext{}, "project_onboarding"},
		{models.PageContext{ProjectKey: "p"}, "service_onboarding"},
		{models.PageContext{ProjectKey: "p", EnvName: "dev"}, "service_diagnosis"},
		{models.PageContext{ProjectKey: "p", PageType: "workflow_task", TaskID: 1}, "workflow_diagnosis"},
		{models.PageContext{ProjectKey: "p", PageType: "environment", EnvName: "dev"}, "environment_inspection"},
		{models.PageContext{PageType: "efficiency"}, "efficiency_diagnosis"},
	} {
		first, _ := Recommendations(item.page, 0, builtins.Builtins())
		next, _ := Recommendations(item.page, 1, builtins.Builtins())
		if len(first) != 3 || len(next) != 3 || first[0].SkillID != item.first || first[0].SkillID == next[0].SkillID ||
			first[0].Context != item.page {
			t.Fatalf("页面推荐或轮换不正确: %+v %+v", first, next)
		}
	}
}

func TestRecoveryDoesNotRunWhileAdmissionIsRegistering(t *testing.T) {
	s := &AgentService{}
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	if err := s.recoverAbandoned(context.Background()); !errors.Is(err, common.ErrBusy) {
		t.Fatalf("不能误恢复入场中的运行: %v", err)
	}
}

func TestSubmittedOperationNeverBecomesSafeToReplayOnReadFailure(t *testing.T) {
	for _, err := range []error{
		context.Canceled,
		context.DeadlineExceeded,
		&PlatformError{Status: 403},
		errors.New("查询失败"),
	} {
		if got := operationFailureStatus(err, true); got != "unknown" {
			t.Fatalf("已提交后的错误不能视作未执行: %s %v", got, err)
		}
	}

	if operationFailureStatus(context.Canceled, false) != "cancelled" ||
		operationFailureStatus(&PlatformError{Uncertain: true}, false) != "unknown" {
		t.Fatal("提交前取消与不确定写响应需要分别处理")
	}

	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":500,"description":"部分写入失败"}`))
	})
	_, err := platform.business(context.Background(), "user", http.MethodPost, "/api/fixed", nil, nil)
	var failure *PlatformError
	if !errors.As(err, &failure) || !failure.Uncertain {
		t.Fatal("通用错误信封不能证明写入未发生")
	}
}

func TestReconciliationMatchesExactServiceSourceAndFreshness(t *testing.T) {
	now := time.Unix(1000, 0)
	item := models.OperationItem{
		Action:     "create_service",
		ProjectKey: "p",
		Params: common.JSON(models.ServiceInput{
			Name:   "api",
			Source: "yaml_git",
			Git: &models.GitConfig{
				CodehostID: 1,
				Owner:      "owner",
				Namespace:  "team/sub",
				Repo:       "repo",
				Branch:     "main",
				Path:       "deploy/api.yaml",
			},
		}),
	}
	raw := `{"service":{"service_name":"api","product_name":"p","revision":1,"create_time":1000,"codehost_id":1,"repo_owner":"owner","repo_namespace":"team/sub","repo_name":"repo","branch_name":"main","load_path":"deploy/api.yaml"}}`
	matched, failed, err := matchesOperation(item, json.RawMessage(raw), now)
	if !matched || failed || err != nil {
		t.Fatalf("准确资源应可核实: %v %v %v", matched, failed, err)
	}

	for _, altered := range []string{
		strings.Replace(raw, "team/sub", "team/other", 1),
		strings.Replace(raw, "\"revision\":1", "\"revision\":2", 1),
		strings.Replace(raw, "\"create_time\":1000", "\"create_time\":999", 1),
	} {
		if ok, _, err := matchesOperation(item, json.RawMessage(altered), now); ok || err != nil {
			t.Fatalf("已有、更新过或来源不一致的资源不能冒充原写入: %s %v", altered, err)
		}
	}
}

func TestSavedDraftCanBeRestoredByExplicitRootWithoutFalseCompletion(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Hour)
	plan := models.OperationPlan{
		Draft:         &models.OnboardingInput{ProjectKey: "demo", ProjectName: "示例", ProjectType: "k8s"},
		MissingFields: []string{"services"},
	}
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id",
			"request",
			"conversation",
			"status",
			"context",
			"created",
			"finished",
			"error",
			"in",
			"out",
			"skill",
			"root",
			"goal",
			"plan",
			"version",
			"plan_status",
			"expires",
			"answer_version",
			"question",
			"available",
		}).
			AddRow(
				"root",
				"req",
				"c",
				"awaiting_input",
				"{}",
				now,
				now,
				"",
				0,
				0,
				"project_onboarding",
				"root",
				"awaiting_input",
				[]byte(common.JSON(plan)),
				0,
				"awaiting_input",
				nil,
				1,
				nil,
				true,
			)
	}
	mock.ExpectQuery("SELECT r.*FROM ai_run r JOIN ai_conversation").WithArgs("root", "u", "u", cutoff).WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}).AddRow("root", "c", now, "project_onboarding", "root", "awaiting_input"))
	mock.ExpectQuery("SELECT .*FROM ai_run WHERE user_id=.*question_source_run_id IS NULL.*plan_status IN").WillReturnRows(rows())
	metadata := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("root", "{}", "[]")
	}
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(metadata())
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"user"}).
		AddRow("u"))
	mock.ExpectQuery("SELECT skill_id,conversation_id,user_id,created_at,status,skill_execution_id,skill_status").
		WillReturnRows(sqlmock.NewRows([]string{
			"skill",
			"conversation",
			"user",
			"created",
			"status",
			"parent",
			"goal",
		}).
			AddRow("project_onboarding", "c", "u", now, "awaiting_input", "root", "awaiting_input"))
	mock.ExpectExec("UPDATE ai_run SET skill_id=.*skill_execution_id").WithArgs("project_onboarding", "root", "current").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "contribution"}).
		AddRow("project_onboarding", "awaiting_input", nil))
	mock.ExpectExec("UPDATE ai_run SET skill_status=.*usage_contribution").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(metadata())
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run .*FOR UPDATE").WithArgs("current", "u").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).
		AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET resource_scopes").WithArgs(`[{"catalog":"saved_operation"}]`, "current", "u").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	reads := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/authorization/auth-info" || r.Header.Get("Authorization") != "user-token" {
			t.Errorf("恢复草稿只读重验当前用户的创建权限: %s %s", r.Method, r.URL)
		}

		w.Write([]byte(`{"is_system_admin":true}`))
	})
	s := &AgentService{Store: store, Platform: platform}
	e := &execution{
		Run:           &models.Run{ID: "current", ConversationID: "c"},
		User:          "u",
		RootID:        "root",
		Skills:        builtins.Builtins(),
		Authorization: "user-token",
		Cutoff:        cutoff,
	}
	value, err := s.readSavedOperation(context.WithValue(context.Background(), executionKey{}, e), struct{}{})
	if err != nil || !strings.Contains(string(common.JSON(value)), "示例") || e.Outcome != "awaiting_input" || e.EvidenceRead.Load() ||
		reads != 1 {
		t.Fatalf("恢复草稿不能计为目标完成: %+v %v %+v", value, err, e)
	}
}

func TestContextDropsTruncatedToolTurn(t *testing.T) {
	now := time.Now()
	question := historyEvent("r", "user", "问题", now)
	call := historyEvent("r", "assistant", "", now)
	call.Choices[0].Message.ToolCalls = []model.ToolCall{{ID: "call"}}
	result := historyEvent("r", "tool", "查询结果", now)
	result.Choices[0].Message.ToolID = "call"
	groups := map[string]string{"r": "r"}
	for _, partial := range [][]event.Event{{question, call}, {question, result}, {call, result}} {
		if got := completeContextTurns(partial, groups); len(got) != 0 {
			t.Fatal("缺少原问题、工具调用或工具结果的轮次不能进入模型")
		}
	}

	if got := completeContextTurns([]event.Event{question, call, result}, groups); len(got) != 3 {
		t.Fatal("完整工具轮次应保留")
	}
}

func TestExecutorNeverSubmitsPreservedSuccess(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT id FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("root"))
	mock.ExpectQuery("SELECT status FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status"}).
		AddRow("executing"))
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "contribution"}).
		AddRow("project_onboarding", "pending_confirmation", nil))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET skill_status=.*usage_contribution").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET status=.*plan_status=.*finished_at").WithArgs("succeeded", "succeeded", sqlmock.AnyArg(), sqlmock.AnyArg(), "retry").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	s := &AgentService{Store: store}
	s.wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	e := &execution{Run: &models.Run{ID: "retry", ConversationID: "c"}, User: "u", RootID: "root"}
	// 不提供业务客户端，任何意外的读取或写入都会使测试失败。
	s.executePlan(
		ctx,
		cancel,
		e,
		models.OperationPlan{Items: []models.OperationItem{{ID: "done", Action: "create_project", Status: "succeeded"}}},
	)
	s.wg.Wait()
}

func TestCleanupConfigurationFailureKeepsLastBacklogEvidence(t *testing.T) {
	store, _ := mockStore(t)
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	p.LoadRuntime = func(context.Context) (*types.AIAssistantRuntime, error) {
		return nil, errors.New("配置数据库不可用")
	}
	old := time.Now().AddDate(0, 0, -100)
	s := &AgentService{
		Store:       store,
		Platform:    p,
		maintenance: models.MaintenanceStatus{OldestExpiredAt: &old, BacklogOver24h: true},
	}
	s.cleanupPass(context.Background())
	if s.maintenance.LastAttemptAt == nil || s.maintenance.LastSuccessAt != nil ||
		s.maintenance.PauseReason != "configuration_unavailable" ||
		!s.maintenance.BacklogOver24h ||
		s.maintenance.OldestExpiredAt != &old {
		t.Fatalf("配置或探测失败不能清除积压证据: %+v", s.maintenance)
	}
}

func TestRetryNoticeUsesNativeMessagesWithoutModelUsage(t *testing.T) {
	store, mock := mockStore(t)
	s := &AgentService{Store: store}
	mem := inmemory.NewSessionService(inmemory.WithGetSessionHook(s.sessionContext), inmemory.WithAppendEventHook(s.beforePersist))
	defer mem.Close()
	s.Sessions = mem
	mock.ExpectExec("UPDATE ai_run SET answer_available=TRUE").WithArgs("retry", "u").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := s.persistRetryNotice(context.Background(), "u", &models.Run{ID: "retry", ConversationID: "c"}); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), executionKey{}, &execution{PersistingSnapshot: true})
	saved, err := mem.GetSession(ctx, session.Key{AppName: "halman", UserID: "u", SessionID: "c"})
	if err != nil || saved == nil || len(saved.Events) != 2 {
		t.Fatalf("重试提示应落到框架消息表: %+v %v", saved, err)
	}

	for _, item := range saved.Events {
		if item.RequestID != "retry" || item.Usage != nil {
			t.Fatalf("无需模型的提示不能制造模型用量，且必须关联新运行: %+v", item)
		}
	}
}
