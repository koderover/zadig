package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"k8s.io/apimachinery/pkg/util/validation"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func missingOnboardingOptions(args models.OnboardingInput, newProject bool) []string {
	missing := []string{}
	for i, build := range args.Builds {
		if build.TemplateID == "" {
			continue
		}
		label := build.Name
		if label == "" {
			label = fmt.Sprintf("第 %d 个构建", i+1)
			missing = append(missing, label+" 的名称")
		}
		if !newProject {
			for _, target := range build.ComponentTargets() {
				if target.ServiceName == "" {
					missing = append(missing, "构建 "+label+" 关联的服务名称")
				} else if target.ServiceModule == "" && hasNewService(args.Services, target.ServiceName) {
					missing = append(missing, "构建 "+label+" 关联的服务 "+target.ServiceName+" 的组件名称")
				}
			}
		}
		if len(build.Repositories) == 0 {
			missing = append(missing, "构建 "+label+" 的源码仓库及分支")
		}
		for _, repo := range build.Repositories {
			missing = append(missing, missingGit(repo, "构建 "+label, false)...)
		}
	}
	return append(missing, missingOnboardingEnvironment(args, newProject)...)
}

func missingOnboardingEnvironment(args models.OnboardingInput, newProject bool) []string {
	if args.Environment == nil {
		if newProject && !args.SkipEnvironment {
			return []string{"请选择：不创建环境、只创建环境不部署服务、或创建环境并部署指定服务。创建环境需提供环境名称、集群和镜像仓库；只创建环境时 services 传空列表，不创建环境时明确跳过"}
		}
		return nil
	}
	missing := []string{}
	for _, field := range []struct{ value, label string }{
		{args.Environment.Name, "环境名称"},
		{args.Environment.ClusterID, "环境的集群"},
		{args.Environment.RegistryID, "环境的镜像仓库"},
	} {
		if field.value == "" {
			missing = append(missing, field.label)
		}
	}
	if args.Environment.Namespace == "" && (!newProject ||
		(args.ProjectKey != "" && args.Environment.Name != "" && suggestedOnboardingNamespace(args) == "")) {
		missing = append(missing, "环境的 Kubernetes namespace")
	}
	return missing
}

func (s *AgentService) resolveOnboardingCluster(ctx context.Context, e *execution, project, value string) (string, error) {
	value = strings.TrimSpace(value)
	if id, err := primitive.ObjectIDFromHex(value); err == nil {
		return id.Hex(), nil
	}

	query := url.Values{"scene": {"selector"}, "production": {"false"}}
	if project != "" {
		query.Set("projectName", project)
	}
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/cluster/clusters", query, nil)
	if err != nil {
		return "", err
	}
	var candidates []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &candidates); err != nil {
		return "", err
	}
	var resolved string
	for _, cluster := range candidates {
		if cluster.Name != value || !primitive.IsValidObjectID(cluster.ID) {
			continue
		}
		if resolved != "" && resolved != cluster.ID {
			return "", errors.New(e.text("有多个集群使用该名称，请在集群目录中确认目标", "Multiple clusters have this name. Confirm the target in the cluster catalog."))
		}
		resolved = cluster.ID
	}
	if resolved == "" {
		return "", errors.New(e.text("未找到可用的目标集群，请核对完整集群名称", "No available cluster matched. Check the full cluster name."))
	}
	return resolved, nil
}

func suggestedOnboardingNamespace(args models.OnboardingInput) string {
	if args.Environment == nil || args.Environment.Namespace != "" || args.Environment.Name == "" || args.ProjectKey == "" {
		return ""
	}
	product := commonmodels.Product{ProductName: args.ProjectKey, EnvName: args.Environment.Name}
	namespace := product.GetDefaultNamespace()
	if len(validation.IsDNS1123Label(namespace)) != 0 {
		return ""
	}
	return namespace
}

// 只用于草稿准备；确认和提交仍读取最新资源。input 必须包含该检查的全部依赖。
func onboardingCheck[T any](e *execution, key string, input any, load func() (T, error)) (T, error) {
	hash := fingerprint(common.JSON([]any{e.User, e.RootID, input}))
	var result T
	if check, ok := e.OnboardingChecks[key]; ok && check.InputHash == hash && json.Unmarshal(check.Result, &result) == nil {
		return result, nil
	}
	result, err := load()
	if err == nil {
		if e.OnboardingChecks == nil {
			e.OnboardingChecks = map[string]models.DraftCheck{}
		}
		e.OnboardingChecks[key] = models.DraftCheck{InputHash: hash, Result: common.JSON(result)}
	}
	return result, err
}

type templateCheck struct {
	ID          string   `json:"-"`
	Fingerprint string   `json:"fingerprint"`
	Missing     []string `json:"missing,omitempty"`
	Credentials []string `json:"credentials,omitempty"`
}

func (s *AgentService) markDraftCredentials(ctx context.Context, e *execution, args *models.OnboardingInput) error {
	for i := range args.Services {
		item := &args.Services[i]
		item.CredentialVariables = nil
		if len(item.Variables) == 0 {
			continue
		}
		if item.TemplateID == "" {
			for key := range item.Variables {
				item.CredentialVariables = append(item.CredentialVariables, key)
			}
			continue
		}
		check, err := s.checkOnboardingTemplate(ctx, e, "service_template/"+item.Name, args.ProjectKey, item.Source, item.TemplateID, item.Variables)
		if err != nil {
			return err
		}
		item.TemplateID = check.ID
		item.CredentialVariables = check.Credentials
	}
	for i := range args.Builds {
		item := &args.Builds[i]
		item.CredentialVariables = nil
		if len(item.Variables) == 0 {
			continue
		}
		if item.TemplateID == "" {
			for key := range item.Variables {
				item.CredentialVariables = append(item.CredentialVariables, key)
			}
			continue
		}
		check, err := s.checkOnboardingTemplate(ctx, e, "build_template/"+item.Name, args.ProjectKey, "build_template", item.TemplateID, item.Variables)
		if err != nil {
			return err
		}
		item.TemplateID = check.ID
		item.CredentialVariables = check.Credentials
	}
	return nil
}

func (s *AgentService) checkOnboardingTemplate(ctx context.Context, e *execution, key, project, source, id string, variables map[string]string) (templateCheck, error) {
	if (source == "yaml_template" || source == "build_template") && !primitive.IsValidObjectID(id) {
		resolved, err := s.onboardingTemplateID(ctx, e, project, source, id)
		if err != nil {
			return templateCheck{}, err
		}
		id = resolved
	}
	check, err := onboardingCheck(e, key, []any{project, source, id, variables}, func() (templateCheck, error) {
		raw, err := s.readTemplate(ctx, e, project, source, id, nil)
		if err != nil {
			return templateCheck{}, err
		}
		result := templateCheck{Fingerprint: fingerprint(raw)}
		list, _, validationErr := templateVariables(raw, variables)
		for _, variable := range list {
			if credential, _ := variable["is_credential"].(bool); credential || sensitiveName.MatchString(variable["key"].(string)) {
				result.Credentials = append(result.Credentials, variable["key"].(string))
			}
		}
		if err := validationErr; err != nil {
			var missing *missingTemplateVariables
			if !errors.As(err, &missing) {
				return templateCheck{}, err
			}
			result.Missing = missing.Keys
		}
		return result, nil
	})
	check.ID = id
	return check, err
}

func (s *AgentService) onboardingTemplateID(ctx context.Context, e *execution, project, source, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New(e.text("请选择模板或提供完整模板名称", "Select a template or provide its full name."))
	}

	request := models.CatalogRequest{Kind: source + "s", ProjectKey: project, Keyword: name, Page: 1}
	for request.Page != 0 {
		raw, err := s.Platform.catalog(ctx, e.Authorization, request)
		if err != nil {
			return "", err
		}
		var page struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			NextPage int `json:"next_page"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		if len(page.Items) > 0 {
			// YAML 和构建模板名称在各自的集合中有唯一索引。
			if len(page.Items) != 1 || !primitive.IsValidObjectID(page.Items[0].ID) {
				return "", errors.New(e.text("模板目录返回的匹配项无效，请重新选择模板", "The template catalog returned an invalid match. Select the template again."))
			}
			return page.Items[0].ID, nil
		}
		request.Page = page.NextPage
	}
	return "", fmt.Errorf(e.text("目录中没有匹配 %q 的模板，请核对完整名称", "No template matches %q in the catalog. Check the full name."), name)
}
