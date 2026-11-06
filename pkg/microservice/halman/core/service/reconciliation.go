package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
)

func (s *AgentService) Reconcile(ctx context.Context, user, authorization, id string, version int, cutoff time.Time) (*models.Run, error) {
	source, err := s.Store.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, err
	}

	if source.PlanVersion != version || version < 1 {
		return nil, common.ErrRequestConflict
	}

	if source.Status != "unknown" {
		return source, nil
	}

	var plan models.OperationPlan
	if json.Unmarshal(source.Plan, &plan) != nil || len(plan.Items) == 0 {
		return nil, common.ErrRequestConflict
	}

	if err := s.authorizeRegeneration(ctx, user, authorization, source, cutoff); err != nil {
		return nil, err
	}

	e := &execution{
		Run:           source,
		User:          user,
		Authorization: authorization,
		Cutoff:        cutoff,
	}
	for i := range plan.Items {
		item := &plan.Items[i]
		switch item.Status {
		case "pending", "blocked":
			item.Status = "failed"
			item.Error = "此前未提交的动作，可在核实全部结果后重新确认重试"
		case "unknown", "submitting":
			status, message, err := s.verifyOperationResult(ctx, e, *item)
			if err != nil {
				return nil, err
			}
			item.Status = status
			item.Error = ""
			if status != "succeeded" {
				item.Error = message
			}
			item.Result = common.JSON(map[string]any{"verification": map[string]any{
				"status":  status,
				"message": message,
				"checked_at": time.Now().
					UTC(),
				"checked_by": user,
			}})
		}
	}

	status := planOutcome(plan)
	if err := s.Store.SaveReconciliation(ctx, user, source, plan, status, cutoff); err != nil {
		return nil, err
	}

	return s.Store.GetRun(ctx, user, id, cutoff)
}

func planOutcome(plan models.OperationPlan) string {
	status := "succeeded"
	for _, item := range plan.Items {
		switch item.Status {
		case "unknown", "submitting":
			return "unknown"
		case "succeeded":
		case "cancelled":
			if status == "succeeded" {
				status = "cancelled"
			}
		default:
			status = "failed"
		}
	}

	return status
}

func (s *AgentService) verifyOperationResult(ctx context.Context, e *execution, item models.OperationItem) (string, string, error) {
	status, message := "unknown", "尚无足够证据证明结果，未执行或重放业务操作"
	if item.Action == "create_workflow" {
		return s.verifyDefaultWorkflows(ctx, e, item)
	}
	if item.Action == "restart_service" || item.Action == "rollback_environment" {
		ready, err := s.repairReady(ctx, e, item)
		if err != nil {
			return status, message, err
		}

		if ready {
			return "succeeded", "实际实例及发布记录已核实", nil
		}

		return status, message, nil
	}

	path := ""
	query := url.Values{"projectName": {item.ProjectKey}}
	switch item.Action {
	case "create_project":
		path = "/api/project/products/" + item.ProjectKey
	case "create_service":
		var a models.ServiceInput
		if json.Unmarshal(item.Params, &a) != nil {
			return status, message, common.ErrRequestConflict
		}
		path = "/api/service/services/" + a.Name
	case "create_build":
		var a models.BuildInput
		if json.Unmarshal(item.Params, &a) != nil {
			return status, message, common.ErrRequestConflict
		}
		path = "/api/build/build/" + a.Name
	case "create_environment":
		var a models.EnvironmentInput
		if json.Unmarshal(item.Params, &a) != nil {
			return status, message, common.ErrRequestConflict
		}
		path = "/api/environment/environments/" + a.Name
		query.Set("production", "false")
	default:
		return status, message, common.ErrRequestConflict
	}

	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, path, query, nil)
	if err != nil {
		var p *PlatformError
		if errors.As(err, &p) && p.Status == http.StatusNotFound {
			return status, "目标暂未找到，不能据此断言原写请求已失败；保留待核实", nil
		}

		return status, message, err
	}

	matched, terminalFailure, err := matchesOperation(item, raw, e.Run.CreatedAt)
	if err != nil {
		return status, message, err
	}

	if matched {
		if item.Action == "create_project" {
			var args models.ProjectInput
			if json.Unmarshal(item.Params, &args) != nil {
				return status, message, common.ErrRequestConflict
			}

			for _, admin := range args.Admins {
				raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/v1/authorization/auth-info", url.Values{"uid": {admin}}, nil)
				if err != nil {
					return status, message, err
				}

				var permissions userclient.AuthorizedResources
				if json.Unmarshal(raw, &permissions) != nil {
					return status, message, common.ErrRequestConflict
				}

				project := permissions.ProjectAuthInfo[item.ProjectKey]
				if project == nil || !project.IsProjectAdmin {
					return status, "项目管理员配置尚未验证一致，保留待核实", nil
				}
			}
		}

		return "succeeded", "实际资源与已确认目标匹配，未重复提交", nil
	}

	if terminalFailure {
		return "failed", "平台已返回明确失败终态；重试前仍需重新预检并确认", nil
	}

	return status, "实际资源尚未就绪、配置不匹配或证据不足，保留待核实", nil
}

func matchesOperation(item models.OperationItem, raw json.RawMessage, created time.Time) (bool, bool, error) {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return false, false, fmt.Errorf("资源核实响应无法解析")
	}

	switch item.Action {
	case "create_project":
		var a models.ProjectInput
		if json.Unmarshal(item.Params, &a) != nil {
			return false, false, common.ErrRequestConflict
		}
		want := map[string]any{
			"product_name":    a.Key,
			"project_name":    a.Name,
			"public":          a.Public,
			"product_feature": map[string]any{"deploy_type": a.Feature["deploy_type"]},
		}
		return containsFields(value, want) && freshResource(value, created), false, nil
	case "create_service":
		var a models.ServiceInput
		if json.Unmarshal(item.Params, &a) != nil {
			return false, false, common.ErrRequestConflict
		}
		if nested, ok := value["service"].(map[string]any); ok {
			value = nested
		}
		if !containsFields(value, map[string]any{"service_name": a.Name, "product_name": item.ProjectKey}) || !freshResource(value, created) {
			return false, false, nil
		}
		if revision, ok := value["revision"].(float64); !ok || revision != 1 {
			return false, false, nil
		}
		if a.ValuesGit != nil {
			from, _ := value["create_from"].(map[string]any)
			yamlData, _ := from["yaml_data"].(map[string]any)
			detail, _ := yamlData["source_detail"].(map[string]any)
			if !gitReferenceMatches(detail, a.ValuesGit) {
				return false, false, nil
			}
		}
		switch a.Source {
		case "yaml_git":
			if a.Git == nil {
				return false, false, common.ErrRequestConflict
			}
			g := a.Git
			isDir, _ := value["is_dir"].(bool)
			if isDir != g.IsDir {
				return false, false, nil
			}
			want := map[string]any{
				"codehost_id": float64(g.CodehostID),
				"repo_owner":  g.Owner,
				"repo_name":   g.Repo,
				"branch_name": g.Branch,
				"load_path":   g.Path,
			}
			if g.Namespace != "" {
				want["repo_namespace"] = g.Namespace
			}
			return containsFields(value, want), false, nil
		case "yaml_template":
			return value["template_id"] == a.TemplateID && variablesMatch(value["service_variable_kvs"], a.Variables), false, nil
		case "helm_git":
			if a.Git == nil {
				return false, false, common.ErrRequestConflict
			}
			from, _ := value["create_from"].(map[string]any)
			return gitReferenceMatches(from, a.Git), false, nil
		case "helm_template":
			from, _ := value["create_from"].(map[string]any)
			return from["template_name"] == a.TemplateID && variablesMatch(from["variables"], a.Variables), false, nil
		case "helm_repository":
			from, _ := value["create_from"].(map[string]any)
			return containsFields(
				from,
				map[string]any{
					"chart_repo_name": a.ChartRepo,
					"chart_name":      a.ChartName,
					"chart_version":   a.ChartVersion,
				},
			), false, nil
		}
	case "create_build":
		var a models.BuildInput
		if json.Unmarshal(item.Params, &a) != nil {
			return false, false, common.ErrRequestConflict
		}
		if !containsFields(
			value,
			map[string]any{
				"name":         a.Name,
				"product_name": item.ProjectKey,
				"template_id":  a.TemplateID,
				"source":       "zadig",
			},
		) {
			return false, false, nil
		}
		targets, _ := value["target_repos"].([]any)
		for _, entry := range targets {
			target, _ := entry.(map[string]any)
			service, _ := target["service"].(map[string]any)
			if !containsFields(
				service,
				map[string]any{
					"product_name":   item.ProjectKey,
					"service_name":   a.ServiceName,
					"service_module": a.ServiceModule,
				},
			) {
				continue
			}

			repos, _ := target["repos"].([]any)
			if len(repos) != len(a.Repositories) || !variablesMatch(target["envs"], a.Variables) {
				continue
			}

			matched := true
			for _, g := range a.Repositories {
				found := false
				for _, entry := range repos {
					repo, _ := entry.(map[string]any)
					want := map[string]any{
						"codehost_id": float64(g.CodehostID),
						"repo_owner":  g.Owner,
						"repo_name":   g.Repo,
						"branch":      g.Branch,
					}
					if g.Namespace != "" {
						want["repo_namespace"] = g.Namespace
					}

					if containsFields(repo, want) {
						found = true
					}
				}

				matched = matched && found
			}

			if matched {
				return true, false, nil
			}
		}
	case "create_environment":
		var a models.EnvironmentInput
		if json.Unmarshal(item.Params, &a) != nil {
			return false, false, common.ErrRequestConflict
		}
		if production, _ := value["production"].(bool); production {
			return false, false, nil
		}
		if !containsFields(
			value,
			map[string]any{
				"product_name": item.ProjectKey,
				"env_name":     a.Name,
				"cluster_id":   a.ClusterID,
				"namespace":    a.Namespace,
				"registry_id":  a.RegistryID,
			},
		) ||
			!freshResource(value, created) ||
			!variablesMatch(value["global_variables"], a.Variables) {
			return false, false, nil
		}
		found := map[string]bool{}
		groups, _ := value["services"].([]any)
		for _, group := range groups {
			services, _ := group.([]any)
			for _, entry := range services {
				service, _ := entry.(map[string]any)
				name, _ := service["service_name"].(string)
				found[name] = true
			}
		}
		for _, name := range a.Services {
			if !found[name] {
				return false, false, nil
			}
		}
		state, _ := value["status"].(string)
		return state == "success" || state == "Running", state == "failed", nil
	}

	return false, false, nil
}

func gitReferenceMatches(from map[string]any, g *models.GitConfig) bool {
	if g == nil {
		return false
	}

	git, _ := from["git_repo_config"].(map[string]any)
	want := map[string]any{
		"codehost_id": float64(g.CodehostID),
		"owner":       g.Owner,
		"repo":        g.Repo,
		"branch":      g.Branch,
	}
	if g.Namespace != "" {
		want["namespace"] = g.Namespace
	}

	return containsFields(git, want) && from["load_path"] == g.Path
}

func containsFields(actual, expected map[string]any) bool {
	if actual == nil {
		return false
	}

	for key, want := range expected {
		got, exists := actual[key]
		if !exists {
			return false
		}

		if nested, ok := want.(map[string]any); ok {
			object, _ := got.(map[string]any)
			if !containsFields(object, nested) {
				return false
			}
		} else if string(common.JSON(got)) != string(common.JSON(want)) {
			return false
		}
	}

	return true
}

func variablesMatch(raw any, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}

	list, ok := raw.([]any)
	if !ok {
		return false
	}

	values := map[string]string{}
	for _, entry := range list {
		v, _ := entry.(map[string]any)
		key, _ := v["key"].(string)
		values[key] = fmt.Sprint(v["value"])
	}

	for key, value := range expected {
		if values[key] != value {
			return false
		}
	}

	return true
}

func freshResource(value map[string]any, created time.Time) bool {
	stamp, ok := value["create_time"].(float64)
	return ok && int64(stamp) >= created.Unix()
}
