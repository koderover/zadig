package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/mozillazg/go-pinyin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

var projectKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func checkConfigPath(value string) error {
	if value == "" || len(value) > 1024 || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value ||
		value == ".." ||
		strings.HasPrefix(value, "../") {
		return fmt.Errorf("请选择仓库内明确的配置路径")
	}

	return nil
}

func validateVariables(values map[string]string) error {
	for key, value := range values {
		if key == "" || len(key) > 128 || len(value) > 4096 || sensitiveName.MatchString(key) {
			return fmt.Errorf("变量名、大小或凭据参数不受支持；请引用模板中已有凭据，不在聊天中填写密钥")
		}
	}

	return nil
}

func fingerprint(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// expected 仅在预览时为 nil；确认和提交必须提供指纹，空字符串也要校验。
func (s *AgentService) readTemplate(ctx context.Context, e *execution, project, source, id string, expected *string) ([]byte, error) {
	kind := source
	if kind != "helm_template" && kind != "build_template" {
		kind = "yaml_template"
	}
	raw, err := s.Platform.catalog(ctx, e.Authorization, models.CatalogRequest{Kind: kind, ProjectKey: project, ID: id})
	if err != nil {
		return nil, err
	}
	if expected != nil && fingerprint(raw) != *expected {
		return nil, fmt.Errorf("模板已变化，请重新生成清单")
	}
	return raw, nil
}

func (s *AgentService) prepareProject(ctx context.Context, args models.OnboardingInput) (any, error) {
	return s.prepareOnboarding(ctx, args, true)
}

func (s *AgentService) prepareServices(ctx context.Context, args models.OnboardingInput) (any, error) {
	return s.prepareOnboarding(ctx, args, false)
}

func (s *AgentService) prepareOnboarding(ctx context.Context, args models.OnboardingInput, newProject bool) (any, error) {
	ctx = withReadPhase(ctx)
	if e, ok := ctx.Value(executionKey{}).(*execution); ok && e.RegenerateFrom != nil {
		return nil, fmt.Errorf("重新回答不能生成或改变操作清单")
	}

	skill := "service_onboarding"
	if newProject {
		skill = "project_onboarding"
	}

	e, err := s.skillExecution(ctx, skill)
	if err != nil {
		return nil, err
	}
	if err := s.restoreOnboardingDraft(ctx, e); err != nil {
		return nil, err
	}
	args, err = mergeOnboardingDraft(e.OnboardingDraft, args)
	if err != nil {
		return nil, err
	}

	if err := validateDraftSecrets(args); err != nil {
		return nil, err
	}

	missing := []string{}
	if newProject && args.ProjectName == "" {
		missing = append(missing, "项目展示名称")
	}

	if args.ProjectKey == "" && newProject && args.ProjectName != "" {
		args.ProjectKey = strings.ToLower(strings.Join(pinyin.LazyPinyin(args.ProjectName, pinyin.NewArgs()), "-"))
		args.ProjectKey = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(args.ProjectKey, "-")
		args.ProjectKey = strings.Trim(args.ProjectKey, "-")
		if len(args.ProjectKey) > 32 {
			args.ProjectKey = args.ProjectKey[:32]
		}
	}

	if args.ProjectKey == "" {
		missing = append(missing, "项目标识")
	} else if !projectKeyPattern.MatchString(args.ProjectKey) {
		return nil, fmt.Errorf("项目标识只能包含小写字母、数字和短横线，最长三十二位")
	}

	if args.ProjectType == "" {
		missing = append(missing, "项目类型（K8s YAML 或 Helm；已有项目请从项目信息中读取）")
	} else if args.ProjectType != "k8s" && args.ProjectType != "helm" {
		return nil, fmt.Errorf("只支持 K8s YAML 和 Helm 项目")
	}

	if len(args.Services) == 0 {
		if args.ExpectedServiceCount > 0 {
			missing = append(missing, fmt.Sprintf("本次要新建的 %d 个服务的名称", args.ExpectedServiceCount))
		} else {
			missing = append(missing, "本次要新建的服务名称列表")
		}
	}
	if len(args.Services) > 0 && args.ExpectedServiceCount > 0 && len(args.Services) != args.ExpectedServiceCount {
		missing = append(missing, fmt.Sprintf("本次需要 %d 个服务，当前清单为 %d 个；请补齐服务，或明确调整总数、移除多余服务", args.ExpectedServiceCount, len(args.Services)))
	}

	if len(args.Services) > 20 || len(args.Builds) > 20 {
		return nil, fmt.Errorf("每批最多二十个服务及二十个构建")
	}

	seen := make(map[string]bool)
	for i := range args.Services {
		item := &args.Services[i]
		label := "服务 " + item.Name
		if item.Name == "" {
			label = fmt.Sprintf("第 %d 个服务", i+1)
		}
		if strings.ToLower(item.Name) != item.Name {
			return nil, fmt.Errorf("服务名不能包含大写字母")
		}

		if item.Git != nil && item.Source != "" && item.Source != "yaml_git" && item.Source != "helm_git" {
			return nil, fmt.Errorf("只有仓库来源服务可以指定服务配置 Git 参数")
		}

		if len(item.Variables) > 0 && item.Source != "" && item.Source != "yaml_template" && item.Source != "helm_template" {
			return nil, fmt.Errorf("仅模板来源支持模板变量，仓库来源请使用已有配置")
		}

		if item.Name == "" {
			missing = append(missing, fmt.Sprintf("第 %d 个服务的名称", i+1))
		} else {
			if err := checkNames(item.Name); err != nil {
				return nil, err
			}

			if seen[item.Name] {
				return nil, fmt.Errorf("同批服务不能重名")
			}

			seen[item.Name] = true
		}

		if item.Source == "" {
			missing = append(missing, label+" 的服务配置来源（按项目类型选择代码仓库、服务模板或 Chart 仓库）")
			continue
		}

		if err := validateVariables(item.Variables); err != nil {
			return nil, err
		}

		switch item.Source {
		case "yaml_git", "helm_git":
			if item.Git == nil {
				missing = append(missing, label+" 的仓库、分支及配置路径")
			} else {
				missing = append(missing, missingGit(*item.Git, label, true)...)
			}
		case "yaml_template", "helm_template":
			if item.TemplateID == "" {
				missing = append(missing, label+" 要使用的已有服务模板（不是构建模板）")
			}
		case "helm_repository":
			if item.ChartRepo == "" || item.ChartName == "" || item.ChartVersion == "" {
				missing = append(missing, label+" 的 Chart 仓库、名称及具体版本")
			}
		default:
			return nil, fmt.Errorf("服务来源不受支持；仅使用已有服务配置或服务模板，不从源码生成配置，不 clone 仓库")
		}

		if args.ProjectType == "k8s" && strings.HasPrefix(item.Source, "helm_") ||
			args.ProjectType == "helm" && strings.HasPrefix(item.Source, "yaml_") {
			return nil, fmt.Errorf("服务配置来源与项目类型不一致")
		}

		if item.ValuesGit != nil {
			if !strings.HasPrefix(item.Source, "helm_") {
				return nil, fmt.Errorf("只有 Helm 服务支持 values 文件来源")
			}

			missing = append(missing, missingGit(*item.ValuesGit, label+" values", true)...)
		}
	}

	if len(missing) > 0 {
		return s.saveDraft(ctx, e, args, missing)
	}

	if newProject {
		if len(args.Admins) == 0 {
			args.Admins = []string{e.User}
		}

		found := false
		for _, admin := range args.Admins {
			if admin == e.User {
				found = true
			}
		}

		if !found {
			args.Admins = append(args.Admins, e.User)
		}
	}

	if !newProject {
		project, err := s.Platform.project(ctx, e.Authorization, args.ProjectKey)
		if err != nil {
			return nil, err
		}

		if err := s.recordScope(ctx, e, models.PageContext{PageType: "project", ProjectKey: args.ProjectKey}); err != nil {
			return nil, err
		}

		if project.Type != args.ProjectType {
			return nil, fmt.Errorf("已有项目类型与输入不一致")
		}
	}

	plan := models.OperationPlan{Items: []models.OperationItem{}, Notes: []string{}, Draft: &args}

	dependencies := []string{}
	if newProject {
		plan.Items = append(
			plan.Items,
			models.OperationItem{
				ID:         "project",
				Action:     "create_project",
				ProjectKey: args.ProjectKey,
				Params:     common.JSON(projectInput(args)),
				Status:     "pending",
			},
		)
		dependencies = []string{"project"}
	}

	for i := range args.Services {
		item := &args.Services[i]
		if err := s.preflightService(ctx, e, args.ProjectKey, *item, !newProject); err != nil {
			return nil, err
		}

		if item.TemplateID != "" {
			raw, err := s.readTemplate(ctx, e, args.ProjectKey, item.Source, item.TemplateID, nil)
			if err != nil {
				return nil, err
			}

			item.TemplateFingerprint = fingerprint(raw)
			if err := checkTemplateVariables(raw, item.Variables); err != nil {
				var required *missingTemplateVariables
				if !errors.As(err, &required) {
					return nil, err
				}
				for _, key := range required.Keys {
					missing = append(missing, "服务 "+item.Name+" 的模板必填变量 "+key)
				}
				continue
			}
		}

		id := "service-" + item.Name
		plan.Items = append(
			plan.Items,
			models.OperationItem{
				ID:          id,
				Action:      "create_service",
				ProjectKey:  args.ProjectKey,
				ProjectType: args.ProjectType,
				Params:      common.JSON(item),
				DependsOn:   append([]string{}, dependencies...),
				Status:      "pending",
			},
		)
	}
	if len(missing) > 0 {
		return s.saveDraft(ctx, e, args, missing)
	}

	serviceIDs := make([]string, 0, len(args.Services))
	for _, item := range args.Services {
		serviceIDs = append(serviceIDs, "service-"+item.Name)
	}

	seenBuild := map[string]bool{}
	for i := range args.Builds {
		build := &args.Builds[i]
		if build.TemplateID == "" {
			plan.Notes = append(plan.Notes, fmt.Sprintf(e.text("构建 %s 未选择已有模板，按规则跳过", "Build %s has no existing template selected and will be skipped"), build.Name))
			continue
		}

		if build.Name != "" && seenBuild[build.Name] {
			return nil, fmt.Errorf("同一批构建名称不能重复")
		}

		if build.Name != "" {
			seenBuild[build.Name] = true
		}
		label := build.Name
		if label == "" {
			label = fmt.Sprintf("第 %d 个构建", i+1)
		}
		if build.Name == "" {
			missing = append(missing, label+" 的名称")
		}
		if build.ServiceName == "" {
			missing = append(missing, "构建 "+label+" 关联的服务名称")
		}
		if build.ServiceModule == "" {
			missing = append(missing, "构建 "+label+" 关联的服务组件名称")
		}
		if build.Name == "" || build.ServiceName == "" || build.ServiceModule == "" {
			continue
		}
		if err := checkNames(build.Name, build.ServiceName, build.ServiceModule); err != nil {
			return nil, err
		}

		if !seen[build.ServiceName] {
			missing = append(missing, "构建 "+build.Name+" 关联的服务 "+build.ServiceName+" 不在本次新建服务清单中；当前初始化流程仅支持为本批新建服务附带创建构建，不支持仅创建构建")
			continue
		}

		if err := validateVariables(build.Variables); err != nil {
			return nil, err
		}

		raw, err := s.readTemplate(ctx, e, args.ProjectKey, "build_template", build.TemplateID, nil)
		if err != nil {
			return nil, err
		}

		build.TemplateFingerprint = fingerprint(raw)
		if err := checkTemplateVariables(raw, build.Variables); err != nil {
			var required *missingTemplateVariables
			if !errors.As(err, &required) {
				return nil, err
			}
			for _, key := range required.Keys {
				missing = append(missing, "构建 "+build.Name+" 的模板必填变量 "+key)
			}
			continue
		}

		for _, repo := range build.Repositories {
			fields := missingGit(repo, "构建 "+build.Name, false)
			missing = append(missing, fields...)
			if len(fields) > 0 {
				continue
			}
			if err := validGit(repo, false); err != nil {
				return nil, err
			}

			if err := s.Platform.requireGitSource(ctx, e.Authorization, repo.CodehostID); err != nil {
				return nil, err
			}
		}

		if len(build.Repositories) == 0 {
			missing = append(missing, "构建 "+build.Name+" 的源码仓库及分支")
			continue
		}

		plan.Items = append(
			plan.Items,
			models.OperationItem{
				ID:          "build-" + build.Name,
				Action:      "create_build",
				ProjectKey:  args.ProjectKey,
				ProjectType: args.ProjectType,
				Params:      common.JSON(build),
				DependsOn:   []string{"service-" + build.ServiceName},
				Status:      "pending",
			},
		)
	}
	if len(missing) > 0 {
		return s.saveDraft(ctx, e, args, missing)
	}

	if args.Environment != nil {
		env := args.Environment
		if env.Name == "" || env.ClusterID == "" || env.Namespace == "" || env.RegistryID == "" || len(env.Services) == 0 {
			return s.saveDraft(ctx, e, args, []string{"环境名称、集群、Kubernetes namespace、镜像仓库及待部署服务"})
		}

		if err := checkNames(env.Name, env.Namespace); err != nil {
			return nil, err
		}

		for _, name := range env.Services {
			if !seen[name] {
				return nil, fmt.Errorf("环境只能部署本次已列入清单的服务")
			}
		}

		plan.Items = append(
			plan.Items,
			models.OperationItem{
				ID:          "environment",
				Action:      "create_environment",
				ProjectKey:  args.ProjectKey,
				ProjectType: args.ProjectType,
				Params:      common.JSON(env),
				DependsOn:   serviceIDs,
				Status:      "pending",
			},
		)
		plan.Notes = append(plan.Notes, e.text("创建环境会实际部署所选服务；缺少镜像、Secret 或必要配置会阻塞，不代表已经部署成功", "Creating an environment deploys the selected services. Missing images, Secrets or required configuration may block deployment; creation alone does not mean deployment succeeded"))
	}

	if args.CreateWorkflow {
		deps := []string{}
		for _, item := range plan.Items {
			deps = append(deps, item.ID)
		}

		plan.Items = append(
			plan.Items,
			models.OperationItem{
				ID:          "workflow",
				Action:      "create_workflow",
				ProjectKey:  args.ProjectKey,
				ProjectType: args.ProjectType,
				DependsOn:   deps,
				Status:      "pending",
			},
		)
	}

	if err := s.preflightPermissions(ctx, e, plan); err != nil {
		return nil, err
	}

	return s.savePlan(ctx, e, plan)
}

func missingGit(g models.GitConfig, label string, configuration bool) []string {
	missing := []string{}
	if g.CodehostID < 1 {
		missing = append(missing, label+" 的代码源")
	}
	for _, field := range []struct{ value, name string }{{g.Owner, "仓库组织"}, {g.Repo, "仓库名"}, {g.Branch, "分支"}} {
		if field.value == "" {
			missing = append(missing, label+" 的"+field.name)
		}
	}
	if configuration && g.Path == "" {
		missing = append(missing, label+" 的配置路径")
	}
	return missing
}

type projectDefinition struct {
	Key  string `json:"product_name"`
	Type string
}

func (p *PlatformClient) project(ctx context.Context, auth, key string) (*projectDefinition, error) {
	if err := checkNames(key); err != nil {
		return nil, err
	}

	raw, err := p.business(ctx, auth, http.MethodGet, "/api/project/products/"+key, url.Values{"projectName": {key}}, nil)
	if err != nil {
		return nil, err
	}

	var definition struct {
		Key     string `json:"product_name"`
		Feature struct {
			Type string `json:"deploy_type"`
		} `json:"product_feature"`
	}
	if json.Unmarshal(raw, &definition) != nil || definition.Key != key {
		return nil, fmt.Errorf("项目不存在或归属信息无效")
	}

	return &projectDefinition{Key: key, Type: definition.Feature.Type}, nil
}

func (s *AgentService) preflightService(ctx context.Context, e *execution, project string, item models.ServiceInput, checkExisting bool) error {
	if err := s.Platform.checkConfiguration(ctx, e.Authorization, item); err != nil {
		return err
	}

	for _, git := range []*models.GitConfig{item.Git, item.ValuesGit} {
		if git != nil {
			if err := checkConfigPath(git.Path); err != nil {
				return err
			}

			if err := s.Platform.requireGitSource(ctx, e.Authorization, git.CodehostID); err != nil {
				return err
			}
		}
	}

	if checkExisting {
		raw, err := s.Platform.business(
			ctx,
			e.Authorization,
			http.MethodGet,
			"/api/project/products/"+project+"/services",
			url.Values{"projectName": {project}},
			nil,
		)
		if err != nil {
			return err
		}

		var existing struct {
			Services [][]string `json:"services"`
		}
		if json.Unmarshal(raw, &existing) != nil {
			return fmt.Errorf("已有服务列表无法解析")
		}

		found := false
		for _, group := range existing.Services {
			for _, name := range group {
				if name == item.Name {
					found = true
				}
			}
		}

		if found {
			return fmt.Errorf("服务 %s 已存在，不能覆盖", item.Name)
		}
	}

	return nil
}

func containsName(raw []byte, name string) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}

	var find func(any) bool
	find = func(v any) bool {
		switch x := v.(type) {
		case string:
			return x == name
		case []any:
			for _, entry := range x {
				if find(entry) {
					return true
				}
			}
		case map[string]any:
			for _, key := range []string{"name", "service_name"} {
				if text, ok := x[key].(string); ok && text == name {
					return true
				}
			}
			for _, entry := range x {
				switch entry.(type) {
				case []any, map[string]any:
					if find(entry) {
						return true
					}
				}
			}
		}

		return false
	}
	return find(value)
}

func checkTemplateVariables(raw []byte, values map[string]string) error {
	_, _, err := templateVariables(raw, values)
	return err
}
