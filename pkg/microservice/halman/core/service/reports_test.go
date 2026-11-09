package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

type reportScopeArgument struct{ system bool }

func (a reportScopeArgument) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var scopes []models.ResourceScope
	if json.Unmarshal([]byte(text), &scopes) != nil || len(scopes) != 1 {
		return false
	}
	if a.system {
		return scopes[0].SystemAction == efficiencyInsightAction && scopes[0].ProjectKey == ""
	}
	return scopes[0].ProjectKey == "p" && scopes[0].EnvName == "dev"
}

func expectReportScope(mock sqlmock.Sqlmock, system bool) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), reportScopeArgument{system}, "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func reportExecution(skill string) *execution {
	return &execution{Run: &models.Run{ID: "run", ConversationID: "conversation", CreatedAt: time.Now().UTC(), Context: models.PageContext{PageType: "environment", ProjectKey: "p", EnvName: "dev"}}, User: "user", Authorization: token("user"), SkillID: skill, Skills: builtins.Builtins()}
}

func TestEnvironmentInspectionReusesOriginalReportWithoutDuplicateCalls(t *testing.T) {
	for _, answer := range []string{"发现异常，token=hidden-secret", ""} {
		store, mock := mockStore(t)
		calls := 0
		platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/authorization/auth-info" {
				w.Write([]byte(`{"project_auth_info":{"p":{"env":{"view":true}}}}`))
				return
			}
			if r.Header.Get("Authorization") != token("user") || r.URL.Query().Get("projectName") != "p" || r.URL.Query().Get("production") != "false" {
				t.Fatal("巡检必须使用实际用户与明确目标")
			}
			switch r.URL.Path {
			case "/api/environment/environments/dev/analysis":
				calls++
				if r.Method != http.MethodPost || r.URL.Query().Get("triggerName") != "ai-assistant" || r.URL.Query().Get("mode") != "" {
					t.Fatal("只能调用原巡检，不增加模式或伪装定时任务")
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) < 150*time.Second {
					t.Fatal("原报告应有独立的长等待预算")
				}
				json.NewEncoder(w).Encode(map[string]string{"result": answer})
			default:
				t.Fatalf("意外调用: %s", r.URL)
			}
		})
		e := reportExecution("environment_inspection")
		s := &AgentService{Store: store, Platform: platform}
		ctx := context.WithValue(context.Background(), executionKey{}, e)
		for i := 0; i < 2; i++ {
			expectReportScope(mock, false)
			expectSkillInputSave(mock, e, nil)
			value, err := s.inspectEnvironment(ctx, models.EnvironmentInspectionInput{})
			if err != nil || strings.Contains(string(common.JSON(value)), "hidden-secret") {
				t.Fatalf("原报告必须成功读取并脱敏: %v %v", value, err)
			}
			if answer == "" && (!strings.Contains(string(common.JSON(value)), "no_report") || e.EvidenceRead.Load()) {
				t.Fatal("空报告不能认定健康或任务成功")
			}
		}
		if calls != 1 || (answer != "" && !e.EvidenceRead.Load()) {
			t.Fatal("相同报告只请求一次，有效报告才标记必要证据已取得")
		}
	}
}

func TestEfficiencyReportUsesOnlyInsightPermissionAndAnswer(t *testing.T) {
	store, mock := mockStore(t)
	posts := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			if r.URL.Query().Get("uid") != "user" {
				t.Fatal("不能信任客户端传入的用户身份")
			}
			w.Write(common.JSON(userclient.AuthorizedResources{SystemActions: &userclient.SystemActions{DataCenter: &userclient.DataCenterActions{ViewInsight: true}}}))
		case "/api/stat/v2/ai/analysis":
			posts++
			var body struct {
				Projects []string `json:"project_list"`
				Start    int64    `json:"start_time"`
				End      int64    `json:"end_time"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Projects) != 1 || body.Projects[0] != "指定项目" || body.End-body.Start != 30*86400 {
				t.Fatalf("必须显式传入指定项目与默认三十天范围: %+v", body)
			}
			w.Write([]byte(`{"answer":"构建耗时较高，建议优化缓存","input_data":"private-input","overall_project_prompt":"private-prompt","token_num":999}`))
		default:
			t.Fatalf("不应请求逐项目权限或项目目录: %s", r.URL)
		}
	})
	e := reportExecution("efficiency_diagnosis")
	s := &AgentService{Store: store, Platform: platform}
	expectReportScope(mock, true)
	expectSkillInputSave(mock, e, nil)
	value, err := s.diagnoseEfficiency(context.WithValue(context.Background(), executionKey{}, e), models.EfficiencyDiagnosisInput{Projects: []string{"指定项目"}})
	text := string(common.JSON(value))
	if err != nil || posts != 1 || !e.EvidenceRead.Load() || e.Run.Context.ProjectKey != "" || !strings.Contains(text, "优化缓存") || strings.Contains(text, "private") || strings.Contains(text, "999") {
		t.Fatalf("只应返回原报告，不包含内部输入和估算用量: %s %v", text, err)
	}
	expectReportScope(mock, true)
	expectSkillInputSave(mock, e, nil)
	if _, err := s.diagnoseEfficiency(context.WithValue(context.Background(), executionKey{}, e), models.EfficiencyDiagnosisInput{Projects: []string{"指定项目"}}); err != nil || posts != 1 {
		t.Fatalf("默认时间范围也应在同次运行内稳定，不重复生成报告: %d %v", posts, err)
	}
}

func TestInspectionHistoryRejectsProductionMixing(t *testing.T) {
	store, mock := mockStore(t)
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/authorization/auth-info" {
			w.Write([]byte(`{"project_auth_info":{"p":{"env":{"view":true}}}}`))
			return
		}
		if r.URL.Path != "/api/environment/environments/analysis/history" || r.URL.Query().Get("production") != "false" || r.URL.Query().Get("pageSize") != "5" {
			t.Fatalf("历史必须有明确范围与有限分页: %s", r.URL)
		}
		w.Write([]byte(`{"total":1,"result":[{"project_name":"p","env_name":"dev","production":true,"status":"success","result":"生产报告"}]}`))
	})
	e := reportExecution("environment_inspection")
	expectSkillInputSave(mock, e, nil)
	s := &AgentService{Store: store, Platform: platform}
	value, err := s.environmentInspectionHistory(context.WithValue(context.Background(), executionKey{}, e), models.EnvironmentHistoryInput{})
	if err == nil || value != nil || e.EvidenceRead.Load() {
		t.Fatal("不能返回串入的生产环境报告或计为完成")
	}
}

func TestInspectionServiceEvidenceCannotStartSkill(t *testing.T) {
	e := reportExecution("")
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	value, err := nativeSkillTool(t, &AgentService{}, "environment_service_evidence").Call(ctx, []byte(`{"service_name":"api"}`))
	if err == nil || value != nil || !strings.Contains(err.Error(), "service_diagnosis") || e.SkillID != "" || e.Run.SkillID != "" || e.RootID != "" {
		t.Fatalf("补充证据不能绑定新的巡检，应引导独立日志查询使用服务诊断: %v %v %+v", value, err, e)
	}
}

func TestServiceLogQueryFromEnvironmentPageCompletesWithoutInspection(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("service_diagnosis")
	logs := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatal("查看日志不得发起环境巡检或写操作")
		}
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			w.Write([]byte(`{"project_auth_info":{"p":{"env":{"view":true}}}}`))
		case "/openapi/environments/dev/service/api/pods":
			w.Write([]byte(`{"service_name":"api","pods":[{"pod_name":"api-1","containers":[{"name":"api"}]}]}`))
		case "/api/logs/pods/api-1":
			if r.URL.Query().Get("previous") == "true" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			logs++
			w.Write([]byte(`{"text":"service is ready","truncated":true}`))
		case "/api/environment/kube/pods/api-1/events":
			w.Write([]byte(`[]`))
		case "/api/environment/environments/dev/services/api", "/api/environment/environments/dev/services/api/resources", "/api/environment/environments/dev/version/api":
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("意外调用: %s", r.URL)
		}
	})
	expectReportScope(mock, false)
	expectSkillInputSave(mock, e, nil)
	s := &AgentService{Store: store, Platform: platform}
	value, err := nativeSkillTool(t, s, "service_diagnosis").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(`{"goal":"diagnose","service_name":"api"}`))
	if err != nil || logs != 1 || !e.EvidenceRead.Load() || !diagnosisGoalComplete(e) || e.Outcome != "" || e.Run.Context.PageType != "service" || !strings.Contains(string(common.JSON(value)), "service is ready") {
		t.Fatalf("环境页的服务日志查询应能独立完成，历史容器日志不可用不影响当前日志: %v %v", value, err)
	}
}

func TestInspectionServiceEvidenceKeepsParentSkillAndDoesNotCompleteInspection(t *testing.T) {
	store, mock := mockStore(t)
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/authorization/auth-info" {
			w.Write([]byte(`{"project_auth_info":{"p":{"env":{"view":true}}}}`))
			return
		}
		if r.Method != http.MethodGet {
			t.Fatal("补充服务证据只能只读")
		}
		if strings.HasSuffix(r.URL.Path, "/pods") {
			w.Write([]byte(`{"service_name":"api","pods":[]}`))
			return
		}
		w.Write([]byte(`{}`))
	})
	e := reportExecution("environment_inspection")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET resource_scopes=").WithArgs(reportScopeArgument{false}, "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	s := &AgentService{Store: store, Platform: platform}
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	if _, err := s.environmentServiceEvidence(ctx, models.ServiceTarget{ServiceName: "api"}); err != nil || e.SkillID != "environment_inspection" || e.Run.Context.PageType != "environment" || e.EvidenceRead.Load() {
		t.Fatalf("不得另建服务 Skill 或仅凭补充证据完成巡检: %v %+v", err, e)
	}
	e.InspectedServices = map[string]bool{"a": true, "b": true, "c": true}
	if _, err := s.environmentServiceEvidence(ctx, models.ServiceTarget{ServiceName: "fourth"}); err == nil {
		t.Fatal("应限制补充证据范围")
	}
}

func TestEfficiencyInputRequiresExplicitBoundedProjectsAndTime(t *testing.T) {
	now := time.Now()
	input := models.EfficiencyDiagnosisInput{Projects: []string{"p"}}
	if err := validateEfficiencyInput(&input, now); err != nil || input.EndTime-input.StartTime != 30*86400 {
		t.Fatalf("默认三十天范围错误: %+v %v", input, err)
	}
	for _, invalid := range []models.EfficiencyDiagnosisInput{
		{}, {Projects: make([]string, 11)}, {Projects: []string{" "}},
		{Projects: []string{"p"}, StartTime: now.AddDate(0, 0, -91).Unix(), EndTime: now.Unix()},
		{Projects: []string{"p"}, StartTime: now.Unix(), EndTime: now.Unix() + 1},
		{Projects: []string{"p"}, EndTime: now.Unix()},
	} {
		if err := validateEfficiencyInput(&invalid, now); err == nil {
			t.Fatalf("不能查询隐式全部项目或无界时间: %+v", invalid)
		}
	}
}

func TestEfficiencyPermissionIsRecheckedForContextAndRegeneration(t *testing.T) {
	granted := true
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/authorization/auth-info" {
			t.Fatalf("系统范围不能套用项目权限: %s", r.URL)
		}
		w.Write(common.JSON(userclient.AuthorizedResources{SystemActions: &userclient.SystemActions{DataCenter: &userclient.DataCenterActions{ViewInsight: granted}}, ProjectAuthInfo: map[string]*userclient.ProjectActions{"p": {IsProjectAdmin: true}}}))
	})
	for _, permitted := range []bool{true, false} {
		granted = permitted
		store, mock := mockStore(t)
		now := time.Now()
		scopes := string(common.JSON([]models.ResourceScope{{SystemAction: efficiencyInsightAction}}))
		mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("past", `{"page_type":"efficiency"}`, scopes))
		mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{"id", "origin", "version", "regenerated", "question_available", "latest"}).AddRow("past", "past", 1, false, true, "past"))
		state := session.NewSession("halman", "user", "conversation")
		state.Events = []event.Event{historyEvent("past", "user", "分析指定项目", now), historyEvent("past", "assistant", "原效能报告", now)}
		e := reportExecution("efficiency_diagnosis")
		e.Cutoff = now.Add(-time.Hour)
		s := &AgentService{Store: store, Platform: platform}
		result, err := s.sessionContext(&session.GetSessionContext{Context: context.WithValue(context.Background(), executionKey{}, e)}, func() (*session.Session, error) { return state, nil })
		if err != nil || (len(result.Events) == 2) != permitted {
			t.Fatalf("历史上下文必须重验系统洞察权限: %+v %v", result, err)
		}
		if (len(e.HistoryRunIDs) == 1 && e.HistoryRunIDs[0] == "past") != permitted {
			t.Fatal("只记录真正进入模型窗口的历史来源，后续模型调用仍须复查")
		}
		mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("past", `{}`, scopes))
		err = s.authorizeRegeneration(context.Background(), "user", token("user"), &models.Run{ID: "past", ConversationID: "conversation"}, e.Cutoff)
		if (err == nil) != permitted {
			t.Fatalf("重新回答不能绕过已撤销的洞察权限: %v", err)
		}
	}
}

func TestOriginalReportFailuresDoNotRetryOrBecomeUnknownWrites(t *testing.T) {
	calls := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusInternalServerError) })
	e := reportExecution("environment_inspection")
	s := &AgentService{Platform: platform}
	for i := 0; i < 2; i++ {
		result := s.originalReport(context.Background(), e, "/api/environment/environments/dev/analysis", nil, nil)
		var failure *PlatformError
		if !errors.As(result.Err, &failure) || failure.Uncertain {
			t.Fatalf("报告失败不能误判为资源写入待核实: %v", result.Err)
		}
	}
	if calls != 1 {
		t.Fatal("同次运行的失败报告不能自动重试")
	}
}
