package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestTaskConfigurationUsesExecutionSpecWithoutCredentialsOrCurrentConfig(t *testing.T) {
	task := &commonmodels.WorkflowTask{
		TaskID: 128, CreateTime: 1700000000,
		WorkflowArgs: &commonmodels.WorkflowV4{Name: "current-definition-is-not-a-snapshot"},
		Stages: []*commonmodels.StageTask{{Jobs: []*commonmodels.JobTask{
			{Name: "build-api", JobType: string(config.JobZadigBuild), Status: config.StatusFailed, Timeout: 600, Spec: map[string]any{
				"properties": map[string]any{"build_os": "go-old", "image_id": "old-build-image", "custom_envs": []any{map[string]any{"key": "TOKEN", "value": "secret-env", "is_credential": true}}, "registries": []any{map[string]any{"secret_key": "secret-registry"}}},
				"steps": []any{
					map[string]any{"name": "checkout", "type": "git", "spec": map[string]any{"codehosts": []any{map[string]string{"access_token": "secret-codehost"}}, "repos": []any{map[string]string{"repo_owner": "team", "repo_name": "api", "branch": "release/v1", "commit_id": "commit-old", "oauth_token": "secret-repo", "ssh_key": "secret-key"}}}},
					map[string]any{"name": "install", "type": "tools", "spec": map[string]any{"installs": []any{map[string]any{"name": "go", "version": "1.20", "scripts": []string{"secret-install"}}}}},
					map[string]any{"name": "run", "type": "shell", "spec": map[string]string{"script": "echo secret-script"}},
				},
			}},
			{Name: "deploy-api", JobType: string(config.JobZadigDeploy), Status: config.StatusFailed, Spec: map[string]any{"env": "dev", "service_name": "api", "production": false, "yaml_content": "secret-yaml", "service_and_images": []any{map[string]string{"service_module": "api", "image": "api:v1"}}}},
		}}},
	}
	snapshot := taskConfiguration(task, "")
	raw, err := json.Marshal(snapshot)
	if err != nil || len(snapshot.Jobs) != 2 || snapshot.TaskID != 128 || snapshot.CreatedAt != 1700000000 {
		t.Fatalf("应返回当次任务来源、时间及失败步骤: %s %v", raw, err)
	}
	for _, expected := range []string{"old-build-image", "release/v1", "commit-old", "1.20", "api:v1"} {
		if !strings.Contains(string(raw), expected) {
			t.Fatalf("缺少当次执行参数 %s: %s", expected, raw)
		}
	}
	for _, excluded := range []string{"secret-", "current-definition", "oauth_token", "ssh_key", "custom_envs", "yaml_content"} {
		if strings.Contains(string(raw), excluded) {
			t.Fatalf("不能返回凭据、变量值、原始正文或当前定义: %s", raw)
		}
	}
}

func TestTaskConfigurationBoundsAndMissingSpec(t *testing.T) {
	task := &commonmodels.WorkflowTask{Stages: []*commonmodels.StageTask{{}}}
	for i := 0; i < 5; i++ {
		task.Stages[0].Jobs = append(task.Stages[0].Jobs, &commonmodels.JobTask{Name: fmt.Sprintf("job-%d", i), JobType: string(config.JobZadigBuild), Status: config.StatusFailed})
	}
	snapshot := taskConfiguration(task, "")
	if len(snapshot.Jobs) != 3 || !snapshot.Truncated || snapshot.Jobs[0].Unavailable == "" {
		t.Fatal("默认最多三个失败步骤，缺配置不能编造默认值")
	}
	snapshot = taskConfiguration(task, "job-4")
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Name != "job-4" || snapshot.Truncated {
		t.Fatal("显式步骤应可读取默认窗口之外的配置")
	}
	job := task.Stages[0].Jobs[0]
	job.Spec = map[string]any{}
	snapshot = taskConfiguration(task, "job-0")
	if snapshot.Jobs[0].Unavailable == "" || snapshot.Jobs[0].Spec != nil {
		t.Fatal("空对象不能被解释成一份使用默认值的执行配置")
	}
	job.Spec = map[string]any{"properties": map[string]string{"image_id": strings.Repeat("x", 9<<10)}}
	snapshot = taskConfiguration(task, "job-0")
	if !snapshot.Truncated || snapshot.Jobs[0].Spec != nil || snapshot.Jobs[0].Unavailable == "" {
		t.Fatal("超大配置应明确标记缺失并限制返回体积")
	}
	job.Spec = map[string]any{"properties": map[string]string{"image_id": "old"}, "steps": make([]any, 21)}
	snapshot = taskConfiguration(task, "job-0")
	if snapshot.Jobs[0].Spec["steps_truncated"] != true {
		t.Fatal("步骤列表截断必须显式标记")
	}
}

func TestTaskConfigurationHelmChartKeepsVersionWithoutValues(t *testing.T) {
	task := &commonmodels.WorkflowTask{Stages: []*commonmodels.StageTask{{Jobs: []*commonmodels.JobTask{{Name: "deploy", JobType: string(config.JobZadigHelmChartDeploy), Status: config.StatusFailed, Spec: map[string]any{"env": "production", "production": true, "deploy_helm_chart": map[string]string{"release_name": "api", "chart_name": "app", "chart_version": "1.2.3", "values_yaml": "secret-values", "override_kvs": "secret-overrides"}}}}}}}
	raw, _ := json.Marshal(taskConfiguration(task, ""))
	if !strings.Contains(string(raw), "1.2.3") || !strings.Contains(string(raw), `"production":true`) || strings.Contains(string(raw), "secret-") {
		t.Fatalf("Helm 应保留当次版本与生产属性，不返回 values: %s", raw)
	}
}
