package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestWorkflowConfigurationUsesExistingTaskEndpointAndReachesModelEvidence(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	e.RootID = e.Run.ID
	e.Run.Context = models.PageContext{ProjectKey: "p", WorkflowName: "wf", TaskID: 128}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workflow/v4/workflowtask/workflow/wf/task/128":
			if r.URL.Query().Get("include_configuration") != "true" || r.URL.Query().Get("job_name") != "build-api" {
				t.Fatal("诊断应通过原任务接口请求指定步骤的执行快照")
			}
			w.Write([]byte(`{"project_key":"p","workflow_key":"wf","task_id":128,"status":"failed","stages":[{"name":"build","jobs":[{"name":"build-api","type":"zadig-build","status":"failed"}]}],"configuration":{"source":"workflow_task.stages.jobs.spec","task_id":128,"jobs":[{"name":"build-api","spec":{"branch":"release/v1","note":"token=hidden-secret"}}]}}`))
		case "/api/logs/log/v4/workflow/wf/tasks/128/jobs/build-api":
			w.Write([]byte(`{"text":"build failed"}`))
		default:
			t.Fatalf("不能查询当前工作流、构建模板或环境替代历史配置: %s", r.URL)
		}
	})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectSkillInputSave(mock, e, nil)
	s := &AgentService{Store: store, Platform: platform}
	value, err := s.diagnoseWorkflow(context.WithValue(context.Background(), executionKey{}, e), models.WorkflowTarget{JobName: "build-api"})
	raw := string(common.JSON(value))
	if err != nil || !e.EvidenceRead.Load() || !strings.Contains(raw, "release/v1") || strings.Contains(raw, "hidden-secret") {
		t.Fatalf("模型应取得历史配置及日志，并再次脱敏: %s %v", raw, err)
	}
}

func TestWorkflowConfigurationAuthorizationDoesNotRequestSnapshotAndOversizeIsBounded(t *testing.T) {
	include := false
	absent := false
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Query().Get("include_configuration") == "true") != include {
			t.Fatal("仅诊断需要配置，权限校验不额外读取快照")
		}
		reply := map[string]any{"project_key": "p", "workflow_key": "wf", "task_id": 128}
		if !absent {
			reply["configuration"] = map[string]string{"payload": strings.Repeat("x", 33<<10)}
		}
		w.Write(common.JSON(reply))
	})
	target := models.WorkflowTarget{ProjectKey: "p", WorkflowName: "wf", TaskID: 128}
	value, err := platform.workflowTask(context.Background(), token("user"), target, false)
	if err != nil || value.Configuration != nil {
		t.Fatalf("权限检查不应保留配置内容: %+v %v", value, err)
	}
	include = true
	value, err = platform.workflowTask(context.Background(), token("user"), target, true)
	if err != nil || len(value.Configuration) > 32<<10 || !json.Valid(value.Configuration) || !strings.Contains(string(value.Configuration), "unavailable") {
		t.Fatalf("超大配置不能进入模型，应保留可解释的缺失说明: %+v %v", value, err)
	}
	absent = true
	value, err = platform.workflowTask(context.Background(), token("user"), target, true)
	if err != nil || !strings.Contains(string(value.Configuration), "未返回") {
		t.Fatalf("旧接口或缺失快照须明确降级，不能拿当前配置补齐: %+v %v", value, err)
	}
}
