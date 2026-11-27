package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// The project validator and this builder share the typed draft. Display strings
// in missing_fields stay available to the model for fields that need lookup.
func projectCardFields(args models.OnboardingInput, catalog map[string][]models.UIOption) []models.UIField {
	var fields []models.UIField
	add := func(id, label, kind string, options ...models.UIOption) {
		fields = append(fields, models.UIField{ID: id, Label: label, Type: kind, Options: options, Required: true})
	}
	if args.ProjectName == "" {
		add("project_name", "项目展示名称", "text")
	} else if args.ProjectKey == "" {
		add("project_key", "项目标识", "text")
	}
	if args.ProjectType == "" {
		add("project_type", "项目类型", "radio",
			models.UIOption{Value: "k8s", Label: "K8s YAML"},
			models.UIOption{Value: "helm", Label: "Helm"})
	}
	if len(args.Services) == 0 || args.ExpectedServiceCount > len(args.Services) {
		add("service_names", "请填写要补充的服务名称，每行一个", "textarea")
		return fields
	}
	for i, item := range args.Services {
		field := "service." + strconv.Itoa(i) + "."
		if item.Name == "" {
			// Names are merged in order into unnamed entries.
			add("service_names", "请补充未命名服务的名称，每行一个", "textarea")
			return fields
		}
		if item.Source == "" {
			var options []models.UIOption
			if args.ProjectType == "k8s" {
				options = []models.UIOption{{Value: "yaml_git", Label: "代码仓库"}, {Value: "yaml_template", Label: "YAML 服务模板"}}
			} else if args.ProjectType == "helm" {
				options = []models.UIOption{{Value: "helm_git", Label: "代码仓库"}, {Value: "helm_template", Label: "Helm 服务模板"}, {Value: "helm_repository", Label: "Chart 仓库"}}
			}
			if len(options) > 0 {
				add(field+"source", "服务 "+item.Name+" 的配置来源", "radio", options...)
			}
		}
	}
	if args.Environment == nil && !args.SkipEnvironment {
		var services []models.UIOption
		for _, item := range args.Services {
			services = append(services, models.UIOption{Value: item.Name, Label: item.Name})
		}
		add("environment.choice", "是否创建环境", "radio",
			models.UIOption{Value: "skip", Label: "不创建环境"},
			models.UIOption{Value: "empty", Label: "只创建环境，不部署服务"},
			models.UIOption{Value: "deploy", Label: "创建环境并部署选定服务"})
		add("environment.services", "要部署的服务（仅选择部署时填写）", "checkbox", services...)
		fields[len(fields)-1].Required = false
	} else if env := args.Environment; env != nil {
		if env.Name == "" {
			add("environment.name", "环境名称", "text")
		}
		if env.Namespace == "" {
			add("environment.namespace", "Kubernetes namespace", "text")
		}
		if env.ClusterID == "" && len(catalog["clusters"]) > 0 {
			add("environment.cluster_id", "环境的集群", "radio", catalog["clusters"]...)
		}
		if env.RegistryID == "" && len(catalog["registries"]) > 0 {
			add("environment.registry_id", "环境的镜像仓库", "radio", catalog["registries"]...)
		}
	}
	return fields
}

func (s *AgentService) presentProjectDraft(ctx context.Context, e *execution, args models.OnboardingInput, missing []string) {
	if e.RegenerateFrom != nil || e.UIPending || len(missing) == 0 {
		return
	}
	fields := projectCardFields(args, e.UICatalogOptions)
	if len(fields) == 0 {
		e.UIFallback = e.Language.Text("请补充 "+missing[0]+"。", "Please provide the missing information: "+missing[0]+".")
		return
	}
	if len(fields) > 30 {
		fields = fields[:30]
		// Keep the environment choice and dependent service selection together.
		if fields[len(fields)-1].ID == "environment.choice" {
			fields = fields[:len(fields)-1]
		}
	}
	in := models.UICardInput{
		Title:   e.Language.Text("补充项目初始化参数", "Complete the project setup"),
		Actions: []models.UIAction{{ID: "submit", Type: "submit", Label: e.Language.Text("提交并继续", "Submit and continue")}},
	}
	kind := "form"
	if len(fields) <= 5 {
		var questions []models.UIQuestion
		for _, field := range fields {
			q := models.UIQuestion{ID: field.ID, Title: field.Label, Required: field.Required, Options: field.Options}
			switch field.Type {
			case "text":
				q.Type = "text"
			case "radio":
				q.Type = "single"
			case "checkbox":
				q.Type = "multiple"
			}
			if q.Type == "" || (q.Type != "text" && (len(q.Options) < 2 || len(q.Options) > 8)) {
				break
			}
			questions = append(questions, q)
		}
		if len(questions) == len(fields) {
			kind, in.Data.Questions = "question", questions
		}
	}
	if kind == "form" {
		in.Data.Fields = fields
	}
	if _, err := s.emitUICard(ctx, e, kind, in, "project_onboarding"); err != nil {
		e.UIFallback = e.Language.Text("请补充 "+missing[0]+"。", "Please provide the missing information: "+missing[0]+".")
	}
}

func onboardingCardPatch(source *models.Run, values map[string]any) (models.OnboardingInput, error) {
	var plan models.OperationPlan
	var result models.OnboardingInput
	if json.Unmarshal(source.Plan, &plan) != nil || plan.Draft == nil {
		return result, common.ErrRequestConflict
	}
	patch := map[string]any{}
	environment := map[string]any{}
	var services []map[string]any
	var deploy []string
	for key, value := range values {
		switch key {
		case "project_name", "project_key", "project_type":
			text, ok := value.(string)
			text = strings.TrimSpace(text)
			if !ok || text == "" {
				return result, common.ErrInvalidRequest
			}
			if key == "project_key" {
				if err := validateProjectKey(text); err != nil {
					return result, fmt.Errorf("%w: %s", common.ErrInvalidRequest, err)
				}
			}
			patch[key] = text
		case "environment.name", "environment.namespace", "environment.cluster_id", "environment.registry_id":
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return result, common.ErrInvalidRequest
			}
			text = strings.TrimSpace(text)
			if key == "environment.name" || key == "environment.namespace" {
				if err := checkNames(text); err != nil {
					return result, fmt.Errorf("%w: %s", common.ErrInvalidRequest, err)
				}
			}
			environment[strings.TrimPrefix(key, "environment.")] = text
		case "environment.choice":
			// This choice is applied after processing its dependent fields.
		case "environment.services":
			if json.Unmarshal(common.JSON(value), &deploy) != nil {
				return result, common.ErrInvalidRequest
			}
		case "service_names":
			text, ok := value.(string)
			if !ok {
				return result, common.ErrInvalidRequest
			}
			names := strings.FieldsFunc(text, func(r rune) bool {
				return r == '\n' || r == ',' || r == '，' || r == '、'
			})
			if len(names) == 0 || len(names) > 20 {
				return result, common.ErrInvalidRequest
			}
			seen := map[string]bool{}
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name != strings.ToLower(name) || seen[name] {
					return result, fmt.Errorf("%w: 服务名称不能重复或包含大写字母", common.ErrInvalidRequest)
				}
				if err := checkNames(name); err != nil {
					return result, fmt.Errorf("%w: %s", common.ErrInvalidRequest, err)
				}
				seen[name] = true
				services = append(services, map[string]any{"name": name})
			}
		default:
			if !strings.HasPrefix(key, "service.") || !strings.HasSuffix(key, ".source") {
				return result, common.ErrInvalidRequest
			}
			index, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(key, "service."), ".source"))
			if err != nil || index < 0 || index >= len(plan.Draft.Services) || plan.Draft.Services[index].Name == "" {
				return result, common.ErrInvalidRequest
			}
			services = append(services, map[string]any{"name": plan.Draft.Services[index].Name, "source": value})
		}
	}
	switch values["environment.choice"] {
	case "skip":
		patch["skip_environment"], patch["environment"] = true, nil
	case "empty":
		patch["skip_environment"], environment["services"] = false, []string{}
	case "deploy":
		if len(deploy) == 0 {
			return result, fmt.Errorf("%w: 请选择要部署的服务", common.ErrInvalidRequest)
		}
		patch["skip_environment"], environment["services"] = false, deploy
	}
	if len(environment) > 0 {
		patch["environment"] = environment
	}
	if len(services) > 0 {
		patch["services"] = services
	}
	err := json.Unmarshal(common.JSON(patch), &result)
	return result, err
}

// Catalog picks contain only IDs and display labels from an already authorized
// lookup. A draft with no candidates can still use the normal text follow-up.
func recordCardCatalogOptions(e *execution, kind string, raw []byte) {
	if !e.UICardsEnabled || (kind != "clusters" && kind != "registries") {
		return
	}
	var items []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"reg_addr"`
		Namespace string `json:"namespace"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return
	}
	options := make([]models.UIOption, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		label := item.Name
		if kind == "registries" {
			label = item.Address + "/" + item.Namespace
		}
		if item.ID != "" && strings.Trim(label, " /") != "" && !seen[item.ID] {
			options = append(options, models.UIOption{Value: item.ID, Label: label})
			seen[item.ID] = true
		}
	}
	if e.UICatalogOptions == nil {
		e.UICatalogOptions = map[string][]models.UIOption{}
	}
	e.UICatalogOptions[kind] = options
}
