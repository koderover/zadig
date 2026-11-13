package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func expectSkillInputSave(mock sqlmock.Sqlmock, e *execution, status any, plan ...sqlmock.Argument) {
	planArgument := sqlmock.AnyArg()
	if len(plan) > 0 {
		planArgument = plan[0]
	}
	root := e.RootID
	if root == "" {
		root = e.Run.ID
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation").WithArgs(e.Run.ConversationID, e.User).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(e.Run.ConversationID))
	mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WithArgs(root, e.User, sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"active", "status"}).AddRow(nil, "running"))
	mock.ExpectQuery("SELECT status FROM ai_run").WithArgs(e.Run.ID, e.User, e.Run.ConversationID).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("running"))
	mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=NULL").WithArgs(root, e.User).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET plan=.*plan_status=").WithArgs(planArgument, status, e.Run.ID, e.User).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func nativeSkillTool(t *testing.T, s *AgentService, name string) tool.CallableTool {
	t.Helper()
	for _, item := range s.tools() {
		if item.Declaration().Name == name {
			return item.(tool.CallableTool)
		}
	}
	t.Fatalf("未找到工具 %s", name)
	return nil
}

func TestReadOnlySkillsPersistPartialInputThroughNativeTools(t *testing.T) {
	for _, tc := range []struct{ skill, input, missing string }{
		{"workflow_diagnosis", `{"project_key":"p","workflow_name":"wf"}`, "工作流任务编号"},
		{"service_diagnosis", `{"project_key":"p","env_name":"dev","production":true}`, "服务"},
		{"environment_inspection", `{"env_name":"dev","production":false}`, "项目"},
		{"efficiency_diagnosis", `{"start_time":1700000000,"end_time":1700086400,"question":"分析构建耗时"}`, "项目"},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution(tc.skill)
			e.Run.Context = models.PageContext{}
			e.RootID = e.Run.ID
			e.EvidenceRead.Store(true)
			s := &AgentService{Store: store}
			if tc.skill == "efficiency_diagnosis" {
				s.Platform = testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/api/v1/authorization/auth-info" {
						t.Fatalf("缺参时不得调用报告接口: %s", r.URL)
					}
					w.Write([]byte(`{"is_system_admin":true}`))
				})
				expectReportScope(mock, true)
			}
			expectSkillInputSave(mock, e, "awaiting_input")
			fn := nativeSkillTool(t, s, tc.skill)
			if len(fn.Declaration().InputSchema.Required) != 0 {
				t.Fatal("工具必须允许先提交部分参数")
			}
			value, err := fn.Call(context.WithValue(context.Background(), executionKey{}, e), []byte(tc.input))
			if err != nil || e.Outcome != "awaiting_input" || e.EvidenceRead.Load() || !strings.Contains(string(common.JSON(value)), tc.missing) {
				t.Fatalf("缺参必须保存并补问，不计成功: %v %v", value, err)
			}
			if string(e.SkillInput) != tc.input {
				var got, want any
				json.Unmarshal(e.SkillInput, &got)
				json.Unmarshal([]byte(tc.input), &want)
				if string(common.JSON(got)) != string(common.JSON(want)) {
					t.Fatalf("草稿不应丢字段: %s", e.SkillInput)
				}
			}
		})
	}
}

func expectSavedQueryInput(mock sqlmock.Sqlmock, e *execution, source *models.Run, scopes string) {
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT r.*FROM ai_run r JOIN ai_conversation").WithArgs(e.RootID, e.User, e.User, e.Cutoff).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation", "created", "skill", "root", "status"}).AddRow(e.RootID, e.Run.ConversationID, now, e.SkillID, e.RootID, "awaiting_input"))
	columns := strings.Split("id,request,conversation,status,context,created,finished,error,in,out,skill,root,goal,plan,version,plan_status,expires,answer_version,question,available", ",")
	mock.ExpectQuery("SELECT .*FROM ai_run WHERE user_id=.*question_source_run_id IS NULL").WithArgs(e.User, e.Run.ConversationID, e.RootID, e.Cutoff).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(source.ID, "req", e.Run.ConversationID, "awaiting_input", "{}", now, now, "", 0, 0, e.SkillID, e.RootID, nil, []byte(source.Plan), 0, "awaiting_input", nil, 1, nil, true))
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow(source.ID, "{}", scopes))
}

func TestQueryDraftRestoresAcrossRunsAndIgnoresNewPage(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	e.Run.ID = "next"
	e.RootID = "root"
	e.Cutoff = time.Now().Add(-time.Hour)
	e.Run.Context = models.PageContext{ProjectKey: "other", WorkflowName: "other-workflow", TaskID: 99}
	source := &models.Run{ID: "old", Plan: common.JSON(models.OperationPlan{Input: json.RawMessage(`{"project_key":"p","workflow_name":"wf"}`)})}
	expectSavedQueryInput(mock, e, source, "[]")
	s := &AgentService{Store: store, Platform: testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workflow/v4/workflowtask/workflow/wf/task/7" {
			t.Fatalf("不得使用新页面的任务: %s", r.URL)
		}
		w.Write([]byte(`{"project_key":"p","workflow_key":"wf","task_id":7,"status":"failed","stages":[]}`))
	})}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("next", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "next", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectSkillInputSave(mock, e, nil)
	value, err := nativeSkillTool(t, s, "workflow_diagnosis").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(`{"task_id":7}`))
	if err != nil || !e.EvidenceRead.Load() || e.Outcome != "" || !strings.Contains(string(common.JSON(value)), `"task_id":7`) || !strings.Contains(string(e.SkillInput), `"workflow_name":"wf"`) {
		t.Fatalf("跨轮仅补任务编号应恢复原项目和工作流: %v %v", value, err)
	}
}

func TestQueryInputChangesClearDependentTargetsAndKeepExplicitFalse(t *testing.T) {
	s := &AgentService{}
	e := reportExecution("service_diagnosis")
	e.SkillInput = json.RawMessage(`{"project_key":"p","env_name":"prod","service_name":"api","production":true}`)
	no := false
	input, err := resolveSkillInput(context.Background(), s, e, models.ServiceDiagnosisInput{Production: &no, ServiceName: "api"})
	if err != nil || input.Production == nil || *input.Production || input.EnvName != "prod" {
		t.Fatalf("显式 false 必须覆盖生产属性: %+v %v", input, err)
	}
	input, err = resolveSkillInput(context.Background(), s, e, models.ServiceDiagnosisInput{ProjectKey: "another"})
	if err != nil || input.EnvName != "" || input.ServiceName != "" || input.Production != nil {
		t.Fatalf("换项目必须重新确定下级目标: %+v %v", input, err)
	}
	e.SkillID = "workflow_diagnosis"
	e.SkillInput = json.RawMessage(`{"project_key":"p","workflow_name":"old","task_id":7,"job_name":"build"}`)
	wf, err := resolveSkillInput(context.Background(), s, e, models.WorkflowTarget{WorkflowName: "new"})
	if err != nil || wf.TaskID != 0 || wf.JobName != "" || wf.ProjectKey != "p" {
		t.Fatalf("换工作流不能沿用任务编号或步骤: %+v %v", wf, err)
	}
	e.SkillID = "efficiency_diagnosis"
	e.SkillInput = json.RawMessage(`{"project_list":["a","b"],"start_time":1700000000,"end_time":1700086400}`)
	eff, err := resolveSkillInput(context.Background(), s, e, models.EfficiencyDiagnosisInput{Projects: []string{"c"}})
	if err != nil || len(eff.Projects) != 1 || eff.Projects[0] != "c" || eff.StartTime != 1700000000 || eff.EndTime != 1700086400 {
		t.Fatalf("项目列表整体替换，未修改的时间范围保留: %+v %v", eff, err)
	}
}

func TestQueryDraftRestorationRechecksRevokedInsightPermission(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("efficiency_diagnosis")
	e.RootID = "root"
	e.Cutoff = time.Now().Add(-time.Hour)
	source := &models.Run{ID: "old", Plan: common.JSON(models.OperationPlan{Input: json.RawMessage(`{"start_time":1700000000}`)})}
	expectSavedQueryInput(mock, e, source, `[{"system_action":"efficiency_over"}]`)
	s := &AgentService{Store: store, Platform: testPlatform(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })}
	if _, err := resolveSkillInput(context.Background(), s, e, models.EfficiencyDiagnosisInput{Projects: []string{"p"}}); err == nil {
		t.Fatal("恢复草稿不能绕过已撤销的效能洞察权限")
	}
}

func TestRegenerationUsesOriginalQueryAndDoesNotSaveDraft(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("service_diagnosis")
	e.RegenerateFrom = &models.Run{ID: "old", ConversationID: "conversation", SkillID: e.SkillID, Plan: common.JSON(models.OperationPlan{Input: json.RawMessage(`{"project_key":"p","env_name":"dev","service_name":"api","production":true}`)})}
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("old", "{}", `[{"project_key":"p","env_name":"dev","production":true}]`))
	s := &AgentService{Store: store, Platform: testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/authorization/auth-info" || r.URL.Query().Get("uid") != "user" {
			t.Fatalf("重新回答先重验原目标权限: %s", r.URL)
		}
		w.Write([]byte(`{"project_auth_info":{"p":{"production_env":{"view":true}}}}`))
	})}
	no := false
	input, err := resolveSkillInput(context.Background(), s, e, models.ServiceDiagnosisInput{ProjectKey: "other", Production: &no})
	if err != nil || input.ProjectKey != "p" || input.ServiceName != "api" || input.Production == nil || !*input.Production {
		t.Fatalf("重新回答不能改变原目标: %+v %v", input, err)
	}
	if _, err := s.saveSkillInput(context.Background(), e, input, nil); err != nil {
		t.Fatal(err)
	}
}

func TestServiceOnboardingDraftWithoutProjectCanBeContinued(t *testing.T) {
	store, mock := mockStore(t)
	source := &models.Run{ID: "old", ConversationID: "conversation", SkillID: "service_onboarding", Plan: common.JSON(models.OperationPlan{Draft: &models.OnboardingInput{ExpectedServiceCount: 3}})}
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("old", "{}", "[]"))
	s := &AgentService{Store: store}
	if err := s.authorizeRegeneration(context.Background(), "user", "", source, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("尚未提供项目的草稿不能查询空项目而阻止补问: %v", err)
	}
}
