package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func projectInput(a models.OnboardingInput) models.ProjectInput {
	return models.ProjectInput{
		Key:         a.ProjectKey,
		Name:        a.ProjectName,
		Description: a.Description,
		Group:       a.GroupName,
		Admins:      a.Admins,
		Public:      a.Public,
		Enabled:     true,
		Feature: map[string]string{
			"basic_facility":  "kubernetes",
			"deploy_type":     a.ProjectType,
			"create_env_type": "system",
		},
	}
}

func variableList(values map[string]string) []map[string]any {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	list := make([]map[string]any, 0, len(values))
	for _, key := range keys {
		list = append(list, map[string]any{"key": key, "value": values[key], "type": "string"})
	}

	return list
}

func gitQuery(g models.GitConfig) url.Values {
	owner := g.Owner
	if g.Namespace != "" {
		owner = g.Namespace
	}

	return url.Values{
		"repoName":   {g.Repo},
		"repoOwner":  {owner},
		"namespace":  {g.Namespace},
		"branchName": {g.Branch},
	}
}

func gitBody(g models.GitConfig) map[string]any {
	return map[string]any{
		"codehostID": g.CodehostID,
		"owner":      g.Owner,
		"namespace":  g.Namespace,
		"repo":       g.Repo,
		"branch":     g.Branch,
		"paths":      []string{g.Path},
	}
}

// 操作地址和请求结构由程序决定，模型不能提供任意 HTTP 或原始服务配置。
func (s *AgentService) submitOperation(ctx context.Context, e *execution, item models.OperationItem) (json.RawMessage, error) {
	// 提交前必须重读，不能继承预览或确认阶段的读取快照。
	ctx = context.WithValue(ctx, readPhaseKey{}, (*readPhase)(nil))
	query := url.Values{"projectName": {item.ProjectKey}}
	var path string
	var body any
	switch item.Action {
	case "create_project":
		var args models.ProjectInput
		if err := json.Unmarshal(item.Params, &args); err != nil {
			return nil, err
		}
		path = "/api/v1/picket/projects"
		body = args
	case "create_service":
		var args models.ServiceInput
		if err := json.Unmarshal(item.Params, &args); err != nil {
			return nil, err
		}
		if err := s.preflightService(ctx, e, item.ProjectKey, args, true); err != nil {
			return nil, err
		}
		var templateList []map[string]any
		var templateValues map[string]any
		if args.TemplateID != "" {
			raw, err := s.readTemplate(ctx, e, item.ProjectKey, args.Source, args.TemplateID, &args.TemplateFingerprint)
			if err != nil {
				return nil, err
			}

			templateList, templateValues, err = templateVariables(raw, args.Variables)
			if err != nil {
				return nil, err
			}
		}
		switch args.Source {
		case "yaml_git":
			if args.Git == nil {
				return nil, fmt.Errorf("缺少服务配置来源")
			}
			path = "/api/service/loader/load/" + strconv.Itoa(args.Git.CodehostID)
			query = gitQuery(*args.Git)
			query.Set("projectName", item.ProjectKey)
			body = map[string]any{
				"type":          "k8s",
				"product_name":  item.ProjectKey,
				"service_paths": []any{map[string]any{"service_name": args.Name, "path": args.Git.Path, "is_dir": args.Git.IsDir}},
			}
		case "yaml_template":
			text, err := yaml.Marshal(templateValues)
			if err != nil {
				return nil, err
			}
			path = "/api/service/template/load"
			body = map[string]any{
				"service_name":         args.Name,
				"project_name":         item.ProjectKey,
				"template_id":          args.TemplateID,
				"auto_sync":            false,
				"variable_yaml":        string(text),
				"service_variable_kvs": templateList,
			}
		case "helm_git", "helm_repository", "helm_template":
			path = "/api/service/helm/services"
			payload := map[string]any{"name": args.Name, "auto_sync": false, "production": false}
			switch args.Source {
			case "helm_git":
				if args.Git == nil {
					return nil, fmt.Errorf("缺少 Chart 来源")
				}
				payload["source"] = "repo"
				payload["createFrom"] = gitBody(*args.Git)
			case "helm_repository":
				payload["source"] = "chartRepo"
				payload["createFrom"] = map[string]string{
					"chartRepoName": args.ChartRepo,
					"chartName":     args.ChartName,
					"chartVersion":  args.ChartVersion,
				}
			case "helm_template":
				payload["source"] = "chartTemplate"
				payload["createFrom"] = map[string]any{"templateName": args.TemplateID, "variables": templateList}
			}
			if args.ValuesGit != nil {
				g := args.ValuesGit
				repo := gitBody(*g)
				delete(repo, "paths")
				repo["valuesPaths"] = []string{g.Path}
				payload["valuesData"] = map[string]any{"yamlSource": "repo", "autoSync": false, "gitRepoConfig": repo}
			}
			body = payload
		default:
			return nil, fmt.Errorf("服务来源不受支持")
		}
	case "create_build":
		var args models.BuildInput
		if err := json.Unmarshal(item.Params, &args); err != nil {
			return nil, err
		}
		if args.TemplateID == "" {
			return nil, fmt.Errorf("构建必须使用已有模板")
		}
		existing, err := s.Platform.business(
			ctx,
			e.Authorization,
			http.MethodGet,
			"/api/build/build",
			url.Values{"projectName": {item.ProjectKey}, "name": {args.Name}},
			nil,
		)
		if err != nil {
			return nil, err
		}
		if containsName(existing, args.Name) {
			return nil, fmt.Errorf("同名构建已存在，不覆盖")
		}
		modulePath := "/api/service/services/" + args.ServiceName
		if item.ProjectType == "helm" {
			modulePath = "/api/service/helm/" + item.ProjectKey + "/" + args.ServiceName + "/serviceModule"
		}
		modules, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, modulePath, query, nil)
		if err != nil {
			return nil, err
		}
		var service struct {
			Modules []struct {
				Name string `json:"name"`
			} `json:"service_module"`
			HelmModules []struct {
				Name string `json:"name"`
			} `json:"service_modules"`
		}
		if json.Unmarshal(modules, &service) != nil {
			return nil, fmt.Errorf("服务组件读取失败")
		}
		if item.ProjectType == "helm" {
			service.Modules = service.HelmModules
		}
		found := false
		for _, module := range service.Modules {
			if module.Name == args.ServiceModule {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("所选构建组件不在服务配置中，请调整清单")
		}

		if _, err := s.readTemplate(ctx, e, item.ProjectKey, "build_template", args.TemplateID, &args.TemplateFingerprint); err != nil {
			return nil, err
		}

		repos := []any{}
		for _, g := range args.Repositories {
			kind, err := s.Platform.gitSource(ctx, e.Authorization, g.CodehostID)
			if err != nil {
				return nil, err
			}

			repos = append(
				repos,
				map[string]any{
					"source":         kind,
					"repo_owner":     g.Owner,
					"repo_namespace": g.Namespace,
					"repo_name":      g.Repo,
					"branch":         g.Branch,
					"codehost_id":    g.CodehostID,
				},
			)
		}
		path = "/api/build/build"
		body = map[string]any{
			"name":         args.Name,
			"source":       "zadig",
			"pre_build":    map[string]any{"envs": []any{}},
			"template_id":  args.TemplateID,
			"product_name": item.ProjectKey,
			"target_repos": []any{map[string]any{
				"service": map[string]string{
					"product_name":   item.ProjectKey,
					"service_name":   args.ServiceName,
					"service_module": args.ServiceModule,
				},
				"repos": repos,
				"envs":  variableList(args.Variables),
			}},
		}
	case "create_environment":
		var args models.EnvironmentInput
		if err := json.Unmarshal(item.Params, &args); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"product_name":     item.ProjectKey,
			"env_name":         args.Name,
			"cluster_id":       args.ClusterID,
			"namespace":        args.Namespace,
			"registry_id":      args.RegistryID,
			"production":       false,
			"global_variables": variableList(args.Variables),
		}
		if item.ProjectType == "helm" {
			list := []any{}
			for _, name := range args.Services {
				list = append(list, map[string]any{"serviceName": name, "envName": args.Name, "deploy_strategy": "deploy"})
			}

			payload["chartValues"] = list
		} else {
			list := []any{}
			for _, name := range args.Services {
				raw, err := s.Platform.business(
					ctx,
					e.Authorization,
					http.MethodGet,
					"/api/service/services/"+name,
					url.Values{"projectName": {item.ProjectKey}},
					nil,
				)
				if err != nil {
					return nil, err
				}

				var svc struct {
					Service struct {
						Revision int64 `json:"revision"`
					} `json:"service"`
				}
				if json.Unmarshal(raw, &svc) != nil || svc.Service.Revision < 1 {
					return nil, fmt.Errorf("无法读取待部署服务版本")
				}

				list = append(
					list,
					map[string]any{
						"service_name":    name,
						"product_name":    item.ProjectKey,
						"type":            "k8s",
						"revision":        svc.Service.Revision,
						"deploy_strategy": "deploy",
					},
				)
			}

			payload["services"] = []any{list}
		}
		path = "/api/environment/environments"
		query.Set("type", item.ProjectType)
		body = []any{payload}
	case "create_workflow":
		path = "/api/workflow/v4/auto"
	case "restart_service", "rollback_environment":
		var args models.RepairInput
		if err := json.Unmarshal(item.Params, &args); err != nil {
			return nil, err
		}
		if _, err := s.Platform.environment(ctx, e.Authorization, args.Target); err != nil {
			return nil, err
		}
		if err := s.checkRepairSnapshot(ctx, e, args); err != nil {
			return nil, err
		}
		query = repairQuery(args)
		path = "/api/environment/environments/" + args.Target.EnvName
		if item.Action == "restart_service" {
			path += "/services/" + args.Target.ServiceName + "/restart"
		} else {
			path += "/version/" + args.Target.ServiceName + "/rollback"
			raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, path+"/check", query, nil)
			if err != nil {
				return nil, err
			}

			var checked struct {
				Available bool `json:"available"`
			}
			if json.Unmarshal(raw, &checked) != nil || !checked.Available {
				return nil, fmt.Errorf("目标版本预检不允许回滚")
			}

			body = map[string]string{"detail": "AI 助手：用户确认回滚环境版本（" + e.Run.ID + "）"}
		}
	default:
		return nil, fmt.Errorf("操作不受支持")
	}

	if err := s.Store.ClaimOperation(ctx, e.User, e.Run, e.RootID, item.ID); err != nil {
		return nil, err
	}

	return s.Platform.business(ctx, e.Authorization, http.MethodPost, path, query, body)
}
