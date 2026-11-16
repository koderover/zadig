package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestWorkflowDiagnosisUsesOriginalTaskDetails(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	e.RootID = e.Run.ID
	e.Run.Context = models.PageContext{ProjectKey: "p", WorkflowName: "wf", TaskID: 128}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workflow/v4/workflowtask/workflow/wf/task/128":
			if r.URL.RawQuery != "" {
				t.Fatal("原任务详情接口不应增加查询参数")
			}
			w.Write([]byte(`{"project_key":"p","workflow_key":"wf","task_id":128,"create_time":1700000000,"status":"failed","stages":[{"name":"build","jobs":[{"name":"job-0-0-0-构建","type":"zadig-build","status":"failed","spec":{"repos":[{"repo_name":"api","branch":"release/v1","commit_id":"commit-old","oauth_token":"hidden-secret"}],"envs":[{"key":"TOKEN","value":"hidden-secret"}],"script":"hidden-script"}}]}]}`))
		case "/api/logs/log/v4/workflow/wf/tasks/128/jobs/job-0-0-0-构建":
			w.Write([]byte(`{"text":"build failed"}`))
		default:
			t.Fatalf("不能读取当前配置替代历史，也不应增加其他请求: %s", r.URL)
		}
	})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectSkillInputSave(mock, e, nil)
	s := &AgentService{Store: store, Platform: platform}
	value, err := s.diagnoseWorkflow(context.WithValue(context.Background(), executionKey{}, e), models.WorkflowTarget{JobName: "job-0-0-0-构建"})
	raw := string(common.JSON(value))
	if err != nil || !e.EvidenceRead.Load() || !strings.Contains(raw, `"logs":{"job-0-0-0-构建":`) || !strings.Contains(raw, "release/v1") || !strings.Contains(raw, "commit-old") || !strings.Contains(raw, "1700000000") || strings.Contains(raw, "hidden-") {
		t.Fatalf("应从原任务详情中提取分支、提交及时间，不能传递凭据和脚本: %s %v", raw, err)
	}
}

func TestWorkflowJobSpecWhitelistAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		expected    []string
	}{
		{"构建", `{"repos":[{"repo_owner":"team","repo_name":"api","branch":"release/v1","commit_id":"commit-old","oauth_token":"hidden-token","password":"hidden-password"}],"image":"api:v1","envs":[{"key":"TOKEN","value":"hidden-value"}],"steps":[{"type":"shell","script":"hidden-script"}]}`, []string{"release/v1", "commit-old", "api:v1"}},
		{"部署", `{"env":"dev","production":true,"env_alias":"current-alias","service_and_images":[{"service_name":"api","service_module":"main","image":"api:v1","password":"hidden-password"}],"yaml_content":"hidden-yaml","variable_kvs":[{"key":"token","value":"hidden-value"}]}`, []string{"dev", "api:v1"}},
		{"Helm", `{"env":"prod","production":true,"deploy_helm_charts":[{"release_name":"api","chart_name":"app","chart_version":"1.2.3","values_yaml":"hidden-values","override_kvs":"hidden-overrides"}]}`, []string{"prod", "1.2.3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, note := workflowJobSpec(json.RawMessage(tc.input))
			if note != "" {
				t.Fatal(note)
			}
			for _, expected := range tc.expected {
				if !strings.Contains(string(spec), expected) {
					t.Fatalf("未保留原接口已有信息 %s: %s", expected, spec)
				}
			}
			for _, excluded := range []string{"hidden-", "production", "env_alias", "current-alias", "yaml", "script", "envs", "oauth_token", "password"} {
				if strings.Contains(string(spec), excluded) {
					t.Fatalf("不可传递凭据、配置正文及当前环境补充属性: %s", spec)
				}
			}
		})
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"unknown":"hidden-value"}`), json.RawMessage(`[]`), common.JSON(map[string]string{"image": strings.Repeat("x", 9<<10)})} {
		if spec, note := workflowJobSpec(raw); len(spec) != 0 || note == "" {
			t.Fatalf("缺失、不支持或超大配置应明确说明，不能编造默认值: %s %s", spec, note)
		}
	}
	repos := make([]map[string]string, 21)
	for i := range repos {
		repos[i] = map[string]string{"repo_name": fmt.Sprintf("repo-%d", i)}
	}
	spec, note := workflowJobSpec(common.JSON(map[string]any{"repos": repos}))
	var result struct{ Repos []any }
	if json.Unmarshal(spec, &result) != nil || len(result.Repos) != 20 || note == "" {
		t.Fatalf("列表截断应明确说明: %s %s", spec, note)
	}
	spec, _ = workflowJobSpec(json.RawMessage(`{"image":"https://u:hidden-password@example.com/api:v1"}`))
	if strings.Contains(string(spec), "hidden-password") {
		t.Fatalf("白名单字段仍应脱敏: %s", spec)
	}
}

func TestWorkflowTaskOnlyKeepsSelectedJobDetails(t *testing.T) {
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Fatal("步骤筛选仅在 halman 内进行，不增加原接口参数")
		}
		jobs := []map[string]any{}
		for i := 0; i < 5; i++ {
			jobs = append(jobs, map[string]any{"name": fmt.Sprintf("job-%d", i), "status": "failed", "spec": map[string]string{"image": "api:v1"}})
		}
		jobs = append(jobs, map[string]any{"name": "successful", "status": "passed", "spec": map[string]string{"image": "unselected"}})
		w.Write(common.JSON(map[string]any{"project_key": "p", "workflow_key": "wf", "task_id": 128, "stages": []any{map[string]any{"name": "build", "jobs": jobs}}}))
	})
	for _, name := range []string{"", "job-4"} {
		evidence, err := platform.workflowTask(context.Background(), token("user"), models.WorkflowTarget{ProjectKey: "p", WorkflowName: "wf", TaskID: 128, JobName: name})
		if err != nil {
			t.Fatal(err)
		}
		for i, job := range evidence.Stages[0].Jobs {
			selected := name == "" && i < 3 || name == job.Name
			if (len(job.Spec) != 0) != selected {
				t.Fatalf("仅选中步骤可返回有限信息: %s %+v", name, job)
			}
			if name == "" && i >= 3 && i < 5 && job.SpecNote == "" {
				t.Fatal("超过默认窗口时须提示可指定步骤继续查询")
			}
		}
	}
}
