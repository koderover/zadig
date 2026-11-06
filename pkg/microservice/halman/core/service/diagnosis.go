package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

type jobEvidence struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Status   string          `json:"status"`
	Error    string          `json:"error"`
	Spec     json.RawMessage `json:"spec,omitempty"`
	SpecNote string          `json:"spec_note,omitempty"`
}

type stageEvidence struct {
	Name   string        `json:"name"`
	Status string        `json:"status"`
	Error  string        `json:"error"`
	Jobs   []jobEvidence `json:"jobs"`
}

type workflowEvidence struct {
	ProjectKey   string          `json:"project_key"`
	WorkflowName string          `json:"workflow_key"`
	TaskID       int64           `json:"task_id"`
	CreateTime   int64           `json:"create_time,omitempty"`
	Status       string          `json:"status"`
	Error        string          `json:"error"`
	Stages       []stageEvidence `json:"stages"`
}

func (p *PlatformClient) workflowTask(ctx context.Context, authorization string, target models.WorkflowTarget) (*workflowEvidence, error) {
	if err := checkNames(target.ProjectKey, target.WorkflowName); err != nil {
		return nil, err
	}

	if target.TaskID < 1 {
		return nil, fmt.Errorf("需要明确工作流任务编号")
	}

	path := fmt.Sprintf("/api/workflow/v4/workflowtask/workflow/%s/task/%d", target.WorkflowName, target.TaskID)
	raw, err := p.business(ctx, authorization, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var evidence workflowEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return nil, err
	}

	if evidence.ProjectKey != target.ProjectKey || evidence.WorkflowName != target.WorkflowName || evidence.TaskID != target.TaskID {
		return nil, fmt.Errorf("任务与所选项目或工作流不匹配")
	}

	evidence.Error = redactText(evidence.Error)
	selected := 0
	for i := range evidence.Stages {
		evidence.Stages[i].Error = redactText(evidence.Stages[i].Error)
		for j := range evidence.Stages[i].Jobs {
			job := &evidence.Stages[i].Jobs[j]
			job.Error = redactText(job.Error)
			if (target.JobName != "" && job.Name != target.JobName) ||
				(target.JobName == "" && job.Status != "failed" && job.Status != "timeout") {
				job.Spec = nil
				continue
			}
			if selected == 3 {
				job.Spec = nil
				job.SpecNote = "本次仅提取前三个相关步骤的信息，可指定步骤继续查询"
				continue
			}
			selected++
			job.Spec, job.SpecNote = workflowJobSpec(job.Spec)
		}
	}

	return &evidence, nil
}

// 只提取原任务详情已返回的字段，不传递凭据、变量值或当前环境补充属性。
func workflowJobSpec(raw json.RawMessage) (json.RawMessage, string) {
	var spec struct {
		Repos []struct {
			Owner     string `json:"repo_owner,omitempty"`
			Namespace string `json:"repo_namespace,omitempty"`
			Name      string `json:"repo_name,omitempty"`
			Branch    string `json:"branch,omitempty"`
			Tag       string `json:"tag,omitempty"`
			CommitID  string `json:"commit_id,omitempty"`
			PR        int    `json:"pr,omitempty"`
		} `json:"repos,omitempty"`
		Env           string `json:"env,omitempty"`
		ServiceName   string `json:"service_name,omitempty"`
		ServiceModule string `json:"service_module,omitempty"`
		Image         string `json:"image,omitempty"`
		Images        []struct {
			ServiceName   string `json:"service_name,omitempty"`
			ServiceModule string `json:"service_module,omitempty"`
			Image         string `json:"image,omitempty"`
		} `json:"service_and_images,omitempty"`
		Charts []struct {
			ReleaseName  string `json:"release_name,omitempty"`
			ChartRepo    string `json:"chart_repo,omitempty"`
			ChartName    string `json:"chart_name,omitempty"`
			ChartVersion string `json:"chart_version,omitempty"`
		} `json:"deploy_helm_charts,omitempty"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &spec) != nil {
		return nil, "原任务详情未返回可用的步骤配置，仅结合状态和日志分析"
	}
	note := ""
	if len(spec.Repos) > 20 || len(spec.Images) > 20 || len(spec.Charts) > 20 {
		note = "仓库、镜像和 Chart 列表分别最多返回二十项，已截断"
	}
	spec.Repos = spec.Repos[:min(len(spec.Repos), 20)]
	spec.Images = spec.Images[:min(len(spec.Images), 20)]
	spec.Charts = spec.Charts[:min(len(spec.Charts), 20)]
	filtered := common.JSON(spec)
	if string(filtered) == "{}" {
		return nil, "原任务详情未返回可用的步骤配置，仅结合状态和日志分析"
	}
	if len(filtered) > 8<<10 {
		return nil, "步骤信息超过八 KiB，仅结合状态和日志分析"
	}
	value, err := safeJSON(filtered)
	if err != nil {
		return nil, "步骤信息不可用，仅结合状态和日志分析"
	}
	filtered = common.JSON(value)
	if len(filtered) > 8<<10 {
		return nil, "步骤信息超过八 KiB，仅结合状态和日志分析"
	}
	return filtered, note
}

func (s *AgentService) diagnoseWorkflow(ctx context.Context, target models.WorkflowTarget) (any, error) {
	e, err := s.skillExecution(ctx, "workflow_diagnosis")
	if err != nil {
		return nil, err
	}

	target, err = resolveSkillInput(ctx, s, e, target)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	if target.ProjectKey == "" {
		missing = append(missing, "项目")
	}
	if target.WorkflowName == "" {
		missing = append(missing, "工作流")
	}
	if target.TaskID == 0 {
		missing = append(missing, "工作流任务编号")
	} else if target.TaskID < 0 {
		return nil, fmt.Errorf("工作流任务编号必须大于零")
	}
	if len(missing) > 0 {
		return s.saveSkillInput(ctx, e, target, missing)
	}

	evidence, err := s.Platform.workflowTask(ctx, e.Authorization, target)
	if err != nil {
		return nil, err
	}

	if err := s.recordScope(
		ctx,
		e,
		models.PageContext{
			PageType:     "workflow_task",
			ProjectKey:   target.ProjectKey,
			WorkflowName: target.WorkflowName,
			TaskID:       target.TaskID,
			JobName:      target.JobName,
		},
	); err != nil {
		return nil, err
	}

	if _, err := s.saveSkillInput(ctx, e, target, nil); err != nil {
		return nil, err
	}

	logs := make(map[string]any)
	for _, stage := range evidence.Stages {
		for _, job := range stage.Jobs {
			if (target.JobName != "" && job.Name != target.JobName) ||
				(target.JobName == "" && job.Status != "failed" && job.Status != "timeout") {
				continue
			}

			if len(logs) >= 3 {
				break
			}

			if err := checkNames(job.Name); err != nil {
				continue
			}

			query := url.Values{"projectName": {target.ProjectKey}, "format": {"excerpt"}, "offset": {"0"}, "limit": {"200"}}
			path := fmt.Sprintf("/api/logs/log/v4/workflow/%s/tasks/%d/jobs/%s", target.WorkflowName, target.TaskID, job.Name)
			raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, path, query, nil)
			if err != nil {
				logs[job.Name] = map[string]string{"unavailable": err.Error()}
				continue
			}

			value, err := safeJSON(raw)
			if err != nil {
				logs[job.Name] = map[string]string{"unavailable": err.Error()}
			} else {
				logs[job.Name] = value
			}
		}
	}

	e.EvidenceRead.Store(true)
	return map[string]any{
		"task":               evidence,
		"logs":               logs,
		"read_only":          true,
		"configuration_note": "步骤 spec 仅含原任务详情已返回的仓库、分支、提交、服务、镜像或 Chart 信息，缺失或截断时须说明；不能推断工具版本、变量值、脚本或 YAML，也不能把当前环境属性当作历史状态。",
		"source":             fmt.Sprintf("/v1/projects/detail/%s/pipelines/custom/%s/%d", target.ProjectKey, target.WorkflowName, target.TaskID),
	}, nil
}

func (p *PlatformClient) environment(ctx context.Context, authorization string, target models.ServiceTarget) (json.RawMessage, error) {
	if err := checkNames(target.ProjectKey, target.EnvName, target.ServiceName); err != nil {
		return nil, err
	}

	query := serviceQuery(target)
	raw, err := p.business(ctx, authorization, http.MethodGet, "/api/environment/environments/"+target.EnvName, query, nil)
	if err != nil {
		return nil, err
	}

	var env struct {
		Project    string `json:"product_name"`
		Name       string `json:"env_name"`
		Production *bool  `json:"production"`
	}
	if json.Unmarshal(raw, &env) != nil || env.Project != target.ProjectKey || env.Name != target.EnvName ||
		env.Production == nil || *env.Production != target.Production {
		return nil, fmt.Errorf("环境归属或生产属性与所选目标不匹配")
	}

	return raw, nil
}

func (s *AgentService) diagnoseService(ctx context.Context, input models.ServiceDiagnosisInput) (any, error) {
	e, err := s.skillExecution(ctx, "service_diagnosis")
	if err != nil {
		return nil, err
	}

	input, err = resolveSkillInput(ctx, s, e, input)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	if input.Goal == "" {
		missing = append(missing, "本次目标（仅诊断、重启服务或回滚环境版本）")
	} else if input.Goal != "diagnose" && input.Goal != "restart_service" && input.Goal != "rollback_environment" {
		return nil, fmt.Errorf("本次目标只支持仅诊断、重启服务或回滚环境版本")
	}
	if input.ProjectKey == "" {
		missing = append(missing, "项目")
	}
	if input.EnvName == "" {
		missing = append(missing, "环境")
	}
	if input.ServiceName == "" {
		missing = append(missing, "服务")
	}
	if input.Production == nil {
		missing = append(missing, "是否生产环境")
	}
	if len(missing) > 0 {
		return s.saveSkillInput(ctx, e, input, missing)
	}
	target := models.ServiceTarget{ProjectKey: input.ProjectKey, EnvName: input.EnvName, ServiceName: input.ServiceName, Production: *input.Production}

	value, err := s.collectServiceEvidence(ctx, e, target)
	if err != nil {
		return nil, err
	}
	if err := s.recordScope(ctx, e, models.PageContext{PageType: "service", ProjectKey: target.ProjectKey, EnvName: target.EnvName, ServiceName: target.ServiceName, Production: target.Production}); err != nil {
		return nil, err
	}
	var next []string
	if input.Goal != "diagnose" && e.RegenerateFrom == nil {
		next = []string{"生成与本次修复目标一致的确认清单"}
	}
	pending, err := s.saveSkillInput(ctx, e, input, next)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		value.(map[string]any)["next_step"] = pending
	}

	e.EvidenceRead.Store(true)
	return value, nil
}

func diagnosisGoalComplete(e *execution) bool {
	if e.SkillID != "service_diagnosis" || e.RegenerateFrom != nil {
		return true
	}
	var input models.ServiceDiagnosisInput
	return json.Unmarshal(e.SkillInput, &input) == nil && input.Goal == "diagnose"
}

func (s *AgentService) collectServiceEvidence(ctx context.Context, e *execution, target models.ServiceTarget) (any, error) {
	if err := checkNames(target.ProjectKey, target.EnvName, target.ServiceName); err != nil {
		return nil, err
	}
	if err := s.Platform.authorizeEnvironment(ctx, e.Authorization, models.PageContext{ProjectKey: target.ProjectKey, EnvName: target.EnvName, Production: target.Production}); err != nil {
		return nil, err
	}

	query := serviceQuery(target)
	base := "/api/environment/environments/" + target.EnvName + "/services/" + target.ServiceName
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, base, query, nil)
	if err != nil {
		return nil, err
	}

	service, err := safeDiagnosticJSON(raw)
	if err != nil {
		return nil, err
	}

	resources, resourceErr := s.Platform.business(ctx, e.Authorization, http.MethodGet, base+"/resources", query, nil)
	var workload any
	if resourceErr == nil {
		workload, resourceErr = safeDiagnosticJSON(resources)
	}

	versions, versionErr := s.Platform.business(
		ctx,
		e.Authorization,
		http.MethodGet,
		"/api/environment/environments/"+target.EnvName+"/version/"+target.ServiceName,
		query,
		nil,
	)
	var history any
	if versionErr == nil {
		history, versionErr = safeDiagnosticJSON(versions)
	}

	result := map[string]any{
		"service":          service,
		"resources":        workload,
		"release_history":  history,
		"target":           target,
		"runtime_evidence": s.serviceLogs(ctx, e, target),
	}
	if resourceErr != nil {
		result["resources_error"] = resourceErr.Error()
	}

	if versionErr != nil {
		result["history_error"] = versionErr.Error()
	}

	return result, nil
}

func safeDiagnosticJSON(raw json.RawMessage) (any, error) {
	return filterEvidenceJSON(raw, 50, func(key string) bool {
		lower := strings.ToLower(key)
		return strings.Contains(lower, "yaml") || strings.Contains(lower, "script") || sensitiveName.MatchString(key) ||
			lower == "content" || lower == "spec" || lower == "env" || lower == "envs" ||
			lower == "params" || lower == "data" || lower == "stringdata"
	}, sanitized)
}
