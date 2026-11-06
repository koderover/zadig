package workflow

import (
	"encoding/json"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type TaskConfiguration struct {
	Source    string                 `json:"source"`
	TaskID    int64                  `json:"task_id"`
	CreatedAt int64                  `json:"created_at"`
	Jobs      []TaskJobConfiguration `json:"jobs"`
	Truncated bool                   `json:"truncated"`
	Note      string                 `json:"note"`
}

type TaskJobConfiguration struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Timeout     int64          `json:"timeout"`
	Spec        map[string]any `json:"spec,omitempty"`
	Unavailable string         `json:"unavailable,omitempty"`
}

// 只从当次执行记录提取白名单字段，不能用当前工作流、模板或环境补齐历史。
func taskConfiguration(task *commonmodels.WorkflowTask, jobName string) *TaskConfiguration {
	result := &TaskConfiguration{Source: "workflow_task.stages.jobs.spec", TaskID: task.TaskID, CreatedAt: task.CreateTime, Jobs: []TaskJobConfiguration{}, Note: "当次任务执行配置摘要；不包含凭据、变量值、脚本正文或完整 YAML。标识与参数不等于当前环境状态，缺失字段不能推断。"}
	for _, stage := range task.Stages {
		if stage == nil {
			continue
		}
		for _, job := range stage.Jobs {
			if job == nil {
				continue
			}
			if (jobName != "" && job.Name != jobName) || (jobName == "" && job.Status != config.StatusFailed && job.Status != config.StatusTimeout) {
				continue
			}
			if len(result.Jobs) == 3 {
				result.Truncated = true
				return result
			}
			item := TaskJobConfiguration{Name: job.Name, Type: job.JobType, Timeout: job.Timeout}
			item.Spec = executionConfiguration(job)
			if item.Spec == nil {
				item.Unavailable = "该步骤没有可用的执行配置摘要；未使用当前配置替代"
			} else if raw, err := json.Marshal(item.Spec); err != nil || len(raw) > 8<<10 {
				item.Spec = nil
				item.Unavailable = "该步骤的配置摘要超过八 KiB，未返回正文"
				result.Truncated = true
			}
			result.Jobs = append(result.Jobs, item)
		}
	}
	return result
}

func executionConfiguration(job *commonmodels.JobTask) map[string]any {
	if job.Spec == nil {
		return nil
	}
	switch job.JobType {
	case string(config.JobZadigBuild), string(config.JobFreestyle), string(config.JobZadigTesting), string(config.JobZadigScanning):
		var spec struct {
			Properties *commonmodels.JobProperties `json:"properties"`
			Steps      []*commonmodels.StepTask    `json:"steps"`
		}
		if commonmodels.IToi(job.Spec, &spec) != nil || (spec.Properties == nil && len(spec.Steps) == 0) {
			return nil
		}
		p := spec.Properties
		steps := []map[string]any{}
		for _, step := range spec.Steps[:min(len(spec.Steps), 20)] {
			if step == nil {
				continue
			}
			item := map[string]any{"name": step.Name, "type": step.StepType}
			switch step.StepType {
			case config.StepGit:
				var source struct {
					Repos []struct {
						Owner     string `json:"repo_owner"`
						Namespace string `json:"repo_namespace"`
						Name      string `json:"repo_name"`
						Branch    string `json:"branch"`
						Tag       string `json:"tag,omitempty"`
						CommitID  string `json:"commit_id,omitempty"`
						PR        int    `json:"pr,omitempty"`
					} `json:"repos"`
				}
				if commonmodels.IToi(step.Spec, &source) == nil {
					item["repos"] = source.Repos[:min(len(source.Repos), 20)]
					item["repos_truncated"] = len(source.Repos) > 20
				}
			case config.StepTools:
				var tools struct {
					Installs []struct {
						Name    string `json:"name"`
						Version string `json:"version"`
					} `json:"installs"`
				}
				if commonmodels.IToi(step.Spec, &tools) == nil {
					item["installs"] = tools.Installs[:min(len(tools.Installs), 20)]
					item["installs_truncated"] = len(tools.Installs) > 20
				}
			}
			steps = append(steps, item)
		}
		var properties map[string]any
		if p != nil {
			properties = map[string]any{
				"build_os": p.BuildOS, "image_from": p.ImageFrom, "image_id": p.ImageID,
				"cluster_id": p.ClusterID, "namespace": p.Namespace, "timeout": p.Timeout,
				"infrastructure": p.Infrastructure, "res_req_spec": p.ResReqSpec, "cache_enable": p.CacheEnable,
			}
		}
		return map[string]any{
			"properties": properties,
			"steps":      steps, "steps_truncated": len(spec.Steps) > 20,
		}
	case string(config.JobZadigDeploy), string(config.JobZadigHelmDeploy), string(config.JobZadigHelmChartDeploy):
		// 不解码任意 values 或渲染结果，避免把 Secret 和模板变量值带入助手。
		var spec struct {
			Env          string `json:"env"`
			ServiceName  string `json:"service_name"`
			Production   *bool  `json:"production"`
			IsProduction *bool  `json:"is_production"`
			ClusterID    string `json:"cluster_id"`
			ReleaseName  string `json:"release_name"`
			VersionName  string `json:"version_name"`
			Timeout      int    `json:"timeout"`
			Chart        *struct {
				ReleaseName  string `json:"release_name"`
				ChartRepo    string `json:"chart_repo"`
				ChartName    string `json:"chart_name"`
				ChartVersion string `json:"chart_version"`
			} `json:"deploy_helm_chart"`
			Images []struct {
				Module string `json:"service_module"`
				Image  string `json:"image"`
			} `json:"service_and_images"`
			HelmImages []struct {
				Module string `json:"service_module"`
				Image  string `json:"image"`
			} `json:"image_and_service_modules"`
		}
		if commonmodels.IToi(job.Spec, &spec) != nil {
			return nil
		}
		if spec.Env == "" && spec.ServiceName == "" && spec.ClusterID == "" && spec.ReleaseName == "" && len(spec.Images) == 0 && len(spec.HelmImages) == 0 && spec.Chart == nil {
			return nil
		}
		production := spec.Production
		if production == nil {
			production = spec.IsProduction
		}
		return map[string]any{
			"env": spec.Env, "service_name": spec.ServiceName, "production": production,
			"cluster_id": spec.ClusterID, "release_name": spec.ReleaseName, "version_name": spec.VersionName, "timeout": spec.Timeout,
			"service_and_images":        spec.Images[:min(len(spec.Images), 20)],
			"image_and_service_modules": spec.HelmImages[:min(len(spec.HelmImages), 20)],
			"deploy_helm_chart":         spec.Chart,
			"images_truncated":          len(spec.Images) > 20 || len(spec.HelmImages) > 20,
		}
	}
	return nil
}
