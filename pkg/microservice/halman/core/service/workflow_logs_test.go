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

func TestWorkflowDiagnosisSkipsJobsWithoutLogs(t *testing.T) {
	for _, jobName := range []string{"", "监控", "job-0-0-0-监控"} {
		t.Run(jobName, func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution("workflow_diagnosis")
			e.RootID = e.Run.ID
			e.Run.Context = models.PageContext{ProjectKey: "p", WorkflowName: "wf", TaskID: 128}
			logReads := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/workflow/v4/workflowtask/workflow/wf/task/128":
					w.Write([]byte(`{
						"project_key":"p","workflow_key":"wf","task_id":128,"status":"failed",
						"stages":[{"name":"监控","jobs":[
							{"name":"job-0-0-0-监控","display_name":"监控","origin_name":"monitor","type":"grafana","status":"failed",
							 "spec":{"check_time":5,"check_mode":"trigger","alerts":[{"id":"alert-1","name":"High CPU","status":"abnormal","url":"https://grafana.example/hidden-secret"}],"grafana_token":"hidden-secret"}},
							{"name":"approval","type":"approval","status":"failed"},
							{"name":"notification","type":"notification","status":"failed"},
							{"name":"build","type":"zadig-build","status":"failed"}
						]}]
					}`))
				case "/api/logs/log/v4/workflow/wf/tasks/128/jobs/build":
					logReads++
					w.Write([]byte(`{"text":"build failed","total_lines":1}`))
				default:
					t.Fatalf("must not query logs for Grafana or approval jobs: %s", r.URL)
				}
			})
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
			mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			expectSkillInputSave(mock, e, nil)
			s := &AgentService{Store: store, Platform: platform}
			value, err := s.diagnoseWorkflow(context.WithValue(context.Background(), executionKey{}, e), models.WorkflowTarget{JobName: jobName})
			if err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)
			task := result["task"].(*workflowEvidence)
			grafana := task.Stages[0].Jobs[0]
			raw := string(grafana.Spec)
			if !e.EvidenceRead.Load() || grafana.LogSupported || !strings.Contains(raw, `"check_mode":"trigger"`) ||
				!strings.Contains(raw, `"status":"abnormal"`) || strings.Contains(raw, "hidden-secret") {
				t.Fatalf("must retain historical alert results without credentials: %s", raw)
			}
			logs := result["logs"].(map[string]any)
			if jobName == "" {
				if logReads != 1 || len(logs) != 1 || !strings.Contains(string(common.JSON(logs["build"])), "build failed") {
					t.Fatalf("mixed workflows should only read the build log: %+v, reads=%d", logs, logReads)
				}
			} else if logReads != 0 || len(logs) != 0 || task.SelectedJobName != grafana.Name {
				t.Fatalf("a selected Grafana job should use its canonical name and skip logs: %+v, reads=%d", task, logReads)
			}
		})
	}
}

func TestWorkflowJobNameResolutionRejectsAmbiguity(t *testing.T) {
	e := workflowEvidence{Stages: []stageEvidence{{Name: "stage", Jobs: []jobEvidence{
		{Name: "build-1", DisplayName: "构建", OriginName: "build"},
		{Name: "build-2", DisplayName: "构建", OriginName: "build"},
	}}}}
	if name, err := e.resolveJobName("build-1"); err != nil || name != "build-1" {
		t.Fatalf("canonical names should resolve directly: %s %v", name, err)
	}
	for _, name := range []string{"构建", "build", "stage", "missing"} {
		if _, err := e.resolveJobName(name); err == nil {
			t.Fatalf("missing or ambiguous names must not silently select a job: %s", name)
		}
	}
}

func TestGrafanaSpecKeepsBoundedHistoricalResults(t *testing.T) {
	alerts := make([]map[string]string, 21)
	for i := range alerts {
		alerts[i] = map[string]string{"id": "alert", "name": "High CPU", "status": "abnormal", "password": "hidden-secret"}
	}
	raw := common.JSON(map[string]any{"check_time": 0, "check_mode": "alert", "alerts": alerts, "token": "hidden-secret"})
	spec, note := workflowJobSpec(raw, "grafana")
	var result struct {
		CheckTime *int64 `json:"check_time"`
		Alerts    []any  `json:"alerts"`
	}
	if err := json.Unmarshal(spec, &result); err != nil || result.CheckTime == nil || *result.CheckTime != 0 || len(result.Alerts) != 20 || note == "" || strings.Contains(string(spec), "hidden-secret") {
		t.Fatalf("must preserve explicit zero and bounded alerts, strip credentials, and mark truncation: %s %s %v", spec, note, err)
	}
	if spec, _ := workflowJobSpec(raw, "zadig-build"); len(spec) != 0 {
		t.Fatalf("Grafana fields must only be used for Grafana jobs: %s", spec)
	}
	if spec, note := workflowJobSpec(json.RawMessage(`{}`), "grafana"); len(spec) != 0 || note == "" {
		t.Fatalf("missing Grafana results must not be invented: %s %s", spec, note)
	}
}
