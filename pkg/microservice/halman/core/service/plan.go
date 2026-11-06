package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/language"
	"github.com/koderover/zadig/v2/pkg/types"
)

func (s *AgentService) saveDraft(ctx context.Context, e *execution, args models.OnboardingInput, missing []string) (any, error) {
	if e.RegenerateFrom != nil {
		return nil, fmt.Errorf("重新回答不能修改参数草稿")
	}

	if err := validateDraftSecrets(args); err != nil {
		return nil, err
	}

	plan := models.OperationPlan{Items: []models.OperationItem{}, Draft: &args, MissingFields: missing}
	if len(common.JSON(plan)) > 128<<10 {
		return nil, fmt.Errorf("参数草稿过大，请减少本批服务或变量")
	}
	if err := s.Store.SaveDraft(ctx, e.User, e.Run, e.RootID, e.Cutoff, plan); err != nil {
		return nil, err
	}
	e.OnboardingDraft = &args

	e.Outcome = "awaiting_input"
	return map[string]any{
		"status":                 "awaiting_input",
		"missing_fields":         missing,
		"draft":                  args,
		"skill_execution_id":     e.RootID,
		"expected_service_count": args.ExpectedServiceCount,
		"service_count":          len(args.Services),
	}, nil
}

func validateDraftSecrets(args models.OnboardingInput) error {
	for _, item := range args.Services {
		if err := validateVariables(item.Variables); err != nil {
			return err
		}
	}

	for _, item := range args.Builds {
		if err := validateVariables(item.Variables); err != nil {
			return err
		}
	}

	if args.Environment != nil {
		return validateVariables(args.Environment.Variables)
	}

	return nil
}

func (s *AgentService) savePlan(ctx context.Context, e *execution, plan models.OperationPlan) (any, error) {
	if e.RegenerateFrom != nil {
		return nil, fmt.Errorf("重新回答不能生成或修改操作清单")
	}

	plan.Input = e.SkillInput
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}

	if len(raw) > 128<<10 {
		return nil, fmt.Errorf("清单超过允许大小，请减少本批服务数量")
	}

	version, expires, err := s.Store.SavePlan(ctx, e.User, e.Run, e.RootID, e.Cutoff, raw)
	if err != nil {
		return nil, err
	}

	e.Outcome = "pending_confirmation"
	if plan.Draft != nil {
		e.OnboardingDraft = plan.Draft
	}
	card := map[string]any{
		"status":             "pending_confirmation",
		"run_id":             e.Run.ID,
		"version":            version,
		"expires_at":         expires,
		"plan":               plan,
		"skill_execution_id": e.RootID,
	}
	s.publish(e.Run.ID, StreamEvent{Type: "confirmation_required", Data: card})
	return card, nil
}

func (s *AgentService) prepareRepair(ctx context.Context, args models.RepairInput) (any, error) {
	if e, ok := ctx.Value(executionKey{}).(*execution); ok && e.RegenerateFrom != nil {
		return nil, fmt.Errorf("重新回答不能生成或改变操作清单")
	}

	e, err := s.skillExecution(ctx, "service_diagnosis")
	if err != nil {
		return nil, err
	}

	if args.Action != "restart_service" && args.Action != "rollback_environment" {
		return nil, fmt.Errorf("只支持已确认的服务重启和环境版本回滚")
	}
	input, err := resolveSkillInput(ctx, s, e, models.ServiceDiagnosisInput{Goal: args.Action, ProjectKey: args.Target.ProjectKey, EnvName: args.Target.EnvName, ServiceName: args.Target.ServiceName, Production: &args.Target.Production})
	if err != nil {
		return nil, err
	}
	args.Target = models.ServiceTarget{ProjectKey: input.ProjectKey, EnvName: input.EnvName, ServiceName: input.ServiceName, Production: *input.Production}
	if args.Action == "rollback_environment" && args.Revision < 1 {
		return s.saveSkillInput(ctx, e, input, []string{"需要回滚的目标环境历史版本"})
	}
	if _, err := s.saveSkillInput(ctx, e, input, nil); err != nil {
		return nil, err
	}

	if err := s.repairSnapshot(ctx, e, &args); err != nil {
		return nil, err
	}

	if err := s.recordScope(
		ctx,
		e,
		models.PageContext{
			ProjectKey:  args.Target.ProjectKey,
			EnvName:     args.Target.EnvName,
			ServiceName: args.Target.ServiceName,
			Production:  args.Target.Production,
		},
	); err != nil {
		return nil, err
	}

	plan := models.OperationPlan{
		Items: []models.OperationItem{{
			ID:         "repair",
			Action:     args.Action,
			ProjectKey: args.Target.ProjectKey,
			Params:     common.JSON(args),
			Status:     "pending",
		}},
		Notes: []string{e.text("操作受理后可能继续执行，取消聊天不会撤回已提交的操作", "Accepted operations may continue running; cancelling the chat does not undo submitted operations")},
	}
	if err := s.preflightPermissions(ctx, e, plan); err != nil {
		return nil, err
	}

	return s.savePlan(ctx, e, plan)
}

func repairQuery(args models.RepairInput) url.Values {
	return url.Values{
		"projectName": {args.Target.ProjectKey},
		"production":  {strconv.FormatBool(args.Target.Production)},
		"revision":    {strconv.FormatInt(args.Revision, 10)},
		"isHelmChart": {strconv.FormatBool(args.HelmChart)},
	}
}

func (s *AgentService) preflightPermissions(ctx context.Context, e *execution, plan models.OperationPlan) error {
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/v1/authorization/auth-info", url.Values{"uid": {e.User}}, nil)
	if err != nil {
		return err
	}

	var resources userclient.AuthorizedResources
	if err := json.Unmarshal(raw, &resources); err != nil {
		return err
	}

	newProjects := map[string]bool{}
	for _, item := range plan.Items {
		if item.Action == "create_project" {
			if !resources.IsSystemAdmin &&
				(resources.SystemActions == nil || resources.SystemActions.Project == nil || !resources.SystemActions.Project.Create) {
				return fmt.Errorf("当前用户没有创建项目的权限: %w", &PlatformError{Status: http.StatusForbidden})
			}

			newProjects[item.ProjectKey] = true
		}
	}

	for _, item := range plan.Items {
		if resources.IsSystemAdmin || newProjects[item.ProjectKey] {
			continue
		}

		project := resources.ProjectAuthInfo[item.ProjectKey]
		if project == nil {
			return fmt.Errorf("当前用户没有项目 %s 的权限: %w", item.ProjectKey, &PlatformError{Status: http.StatusForbidden})
		}

		if project.IsProjectAdmin {
			continue
		}

		allowed := false
		switch item.Action {
		case "create_service":
			allowed = project.Service != nil && project.Service.Create
		case "create_build":
			allowed = project.Build != nil && project.Build.Create
		case "create_environment":
			allowed = project.Env != nil && project.Env.Create
		case "create_workflow":
			allowed = (project.Workflow != nil && project.Workflow.Create) || (project.Env != nil && project.Env.EditConfig)
		case "restart_service", "rollback_environment":
			var repair models.RepairInput
			if json.Unmarshal(item.Params, &repair) != nil {
				return fmt.Errorf("修复参数无效")
			}
			if repair.Target.Production {
				if project.ProductionEnv != nil {
					if item.Action == "restart_service" {
						allowed = project.ProductionEnv.Restart
					} else {
						allowed = project.ProductionEnv.Rollback
					}
				}
			} else if project.Env != nil {
				if item.Action == "restart_service" {
					allowed = project.Env.Restart
				} else {
					allowed = project.Env.Rollback
				}
			}
		}

		if !allowed && (item.Action == "restart_service" || item.Action == "rollback_environment" || item.Action == "create_workflow") {
			action, path, resourceName := types.EnvActionEditConfig, "/api/v1/authorization/collaboration-action", ""
			if item.Action != "create_workflow" {
				var args models.RepairInput
				if json.Unmarshal(item.Params, &args) != nil {
					return fmt.Errorf("修复参数无效")
				}

				path = "/api/v1/authorization/collaboration-permission"
				resourceName = args.Target.EnvName
				if item.Action == "restart_service" {
					action = types.EnvActionRestart
					if args.Target.Production {
						action = types.ProductionEnvActionRestart
					}
				} else {
					action = types.EnvActionRollback
					if args.Target.Production {
						action = types.ProductionEnvActionRollback
					}
				}
			}

			raw, err := s.Platform.business(
				ctx,
				e.Authorization,
				http.MethodGet,
				path,
				url.Values{
					"uid":           {e.User},
					"project_key":   {item.ProjectKey},
					"resource":      {types.ResourceTypeEnvironment},
					"resource_name": {resourceName},
					"action":        {action},
				},
				nil,
			)
			if err != nil {
				return err
			}

			var result types.CheckCollaborationModePermissionResp
			if json.Unmarshal(raw, &result) == nil && result.Error == "" {
				allowed = result.HasPermission
			}
		}

		if !allowed {
			return fmt.Errorf("当前用户没有操作 %s 的权限: %w", item.Action, &PlatformError{Status: http.StatusForbidden})
		}
	}

	return nil
}

func planError(err error, lang language.Language) string {
	var platform *PlatformError
	if errors.As(err, &platform) {
		return platform.UserMessage(string(lang))
	}

	return lang.Text("操作失败，请检查前置条件及资源实际状态", "Operation failed; check prerequisites and the actual resource state")
}

func (s *AgentService) repairSnapshot(ctx context.Context, e *execution, args *models.RepairInput) error {
	if _, err := s.Platform.environment(ctx, e.Authorization, args.Target); err != nil {
		return err
	}

	pods, err := s.Platform.servicePods(ctx, e.Authorization, args.Target)
	if err != nil {
		return err
	}

	if pods.Truncated {
		return fmt.Errorf("服务实例超过窗口，无法生成完整修复范围")
	}

	args.PodNames = podNames(pods)
	args.PodFingerprint = fingerprint(common.JSON(args.PodNames))
	if len(args.PodNames) == 0 && args.Action == "restart_service" {
		return fmt.Errorf("服务没有可重启的运行实例")
	}

	if args.Action == "rollback_environment" {
		if args.Revision < 1 {
			return fmt.Errorf("必须选择明确的环境历史版本")
		}

		args.CurrentRevision = 0
		versions, err := s.Platform.versions(ctx, e.Authorization, *args)
		if err != nil {
			return err
		}

		var selected any
		for _, version := range versions {
			if version.Revision > args.CurrentRevision {
				args.CurrentRevision = version.Revision
			}

			if version.Revision == args.Revision {
				selected = version
			}
		}

		if selected == nil {
			return fmt.Errorf("所选环境版本不存在")
		}

		args.VersionPreview = common.JSON(selected)
		query := repairQuery(*args)
		raw, err := s.Platform.business(
			ctx,
			e.Authorization,
			http.MethodGet,
			"/api/environment/environments/"+args.Target.EnvName+"/version/"+args.Target.ServiceName+"/rollback/check",
			query,
			nil,
		)
		if err != nil {
			return err
		}

		var result struct {
			CanRollback *bool `json:"available"`
		}
		if json.Unmarshal(raw, &result) != nil || result.CanRollback == nil || !*result.CanRollback {
			return fmt.Errorf("目标版本预检不允许回滚")
		}
	}

	return nil
}
