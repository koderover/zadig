package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
		if build.ServiceName == "" {
			missing = append(missing, "构建 "+label+" 关联的服务名称")
		}
		if build.ServiceModule == "" && (newProject || hasNewService(args.Services, build.ServiceName)) {
			missing = append(missing, "构建 "+label+" 关联的服务组件名称")
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
			return []string{"请选择：不创建环境、只创建环境不部署服务、或创建环境并部署指定服务。创建环境需提供环境名称、集群、Kubernetes namespace 和镜像仓库；只创建环境时 services 传空列表，不创建环境时明确跳过"}
		}
		return nil
	}
	missing := []string{}
	for _, field := range []struct{ value, label string }{
		{args.Environment.Name, "环境名称"},
		{args.Environment.ClusterID, "环境的集群"},
		{args.Environment.Namespace, "环境的 Kubernetes namespace"},
		{args.Environment.RegistryID, "环境的镜像仓库"},
	} {
		if field.value == "" {
			missing = append(missing, field.label)
		}
	}
	return missing
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
		item.CredentialVariables = check.Credentials
	}
	return nil
}

func (s *AgentService) checkOnboardingTemplate(ctx context.Context, e *execution, key, project, source, id string, variables map[string]string) (templateCheck, error) {
	return onboardingCheck(e, key, []any{project, source, id, variables}, func() (templateCheck, error) {
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
}
