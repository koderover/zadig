package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/types"
)

func (s *AgentService) Regenerate(ctx context.Context, user, authorization, id, requestID string, cfg *types.AIAssistantRuntime) (*models.Run, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.HistoryRetentionDays)
	source, err := s.Store.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, err
	}

	if !source.AnswerAvailable {
		return nil, common.ErrRequestConflict
	}

	if source.SkillExecutionID != nil {
		root, err := s.Store.GetRunReference(ctx, user, *source.SkillExecutionID, cutoff)
		if err != nil || root.ConversationID != source.ConversationID {
			return nil, common.ErrNotFound
		}
	}

	origin := source.ID
	if source.QuestionSourceRunID != nil {
		origin = *source.QuestionSourceRunID
	}

	original, err := s.Store.GetRunReference(ctx, user, origin, cutoff)
	if err != nil {
		return nil, err
	}

	if original.ConversationID != source.ConversationID {
		return nil, common.ErrNotFound
	}

	question, err := s.Store.UserQuestion(ctx, user, original)
	if err != nil {
		return nil, err
	}

	if err := s.authorizeRegeneration(ctx, user, authorization, source, cutoff); err != nil {
		return nil, err
	}

	req := models.RunRequest{
		RequestID:      requestID,
		Message:        question,
		Context:        source.Context,
		RegenerateFrom: source,
	}
	if source.SkillExecutionID != nil {
		req.SkillExecutionID = *source.SkillExecutionID
	}

	return s.Start(ctx, user, source.ConversationID, authorization, req, cfg)
}

func (s *AgentService) authorizeRegeneration(ctx context.Context, user, authorization string, source *models.Run, cutoff time.Time) error {
	metadata, err := s.Store.ContextScopes(ctx, user, source.ConversationID, cutoff, []string{source.ID})
	if err != nil {
		return err
	}

	if len(metadata) != 1 {
		return common.ErrNotFound
	}

	scopes := []models.ResourceScope{}
	for _, scope := range metadata[0].Scopes {
		if scope.Catalog != "" {
			continue
		}

		if scope.ProjectKey == "" && scope.SystemAction == "" {
			return common.ErrRequestConflict
		}

		scopes = append(scopes, scope)
	}

	if len(source.Plan) == 0 {
		return s.Platform.authorizeScopes(ctx, authorization, scopes)
	}

	var plan models.OperationPlan
	if json.Unmarshal(source.Plan, &plan) != nil {
		return common.ErrRequestConflict
	}

	pristine := source.PlanStatus != nil &&
		(*source.PlanStatus == "pending_confirmation" || *source.PlanStatus == "awaiting_input" || *source.PlanStatus == "cancelled" ||
			*source.PlanStatus == "failed")
	for _, item := range plan.Items {
		if item.Status == "succeeded" || item.Status == "unknown" || item.Status == "submitting" {
			pristine = false
		}
	}

	newProjects := map[string]bool{}
	if pristine {
		if source.SkillID == "project_onboarding" && plan.Draft != nil {
			newProjects[plan.Draft.ProjectKey] = true
		}

		for _, item := range plan.Items {
			if item.Action == "create_project" {
				newProjects[item.ProjectKey] = true
			}
		}
	}

	if plan.Draft != nil && source.SkillID == "service_onboarding" && plan.Draft.ProjectKey != "" {
		scopes = append(scopes, models.ResourceScope{PageContext: models.PageContext{ProjectKey: plan.Draft.ProjectKey}})
	}

	if len(newProjects) > 0 {
		raw, err := s.Platform.business(ctx, authorization, http.MethodGet, "/api/v1/authorization/auth-info", url.Values{"uid": {user}}, nil)
		if err != nil {
			return err
		}

		var permissions userclient.AuthorizedResources
		if json.Unmarshal(raw, &permissions) != nil {
			return common.ErrRequestConflict
		}

		if !permissions.IsSystemAdmin &&
			(permissions.SystemActions == nil || permissions.SystemActions.Project == nil || !permissions.SystemActions.Project.Create) {
			return &PlatformError{Status: http.StatusForbidden}
		}
	}

	checked := map[string]bool{}
	for _, item := range plan.Items {
		if item.Action == "restart_service" || item.Action == "rollback_environment" {
			var repair models.RepairInput
			if json.Unmarshal(item.Params, &repair) != nil {
				return common.ErrRequestConflict
			}

			scopes = append(scopes, models.ResourceScope{PageContext: models.PageContext{
				ProjectKey: repair.Target.ProjectKey, EnvName: repair.Target.EnvName, Production: repair.Target.Production,
			}})

			continue
		}

		if checked[item.ProjectKey] || newProjects[item.ProjectKey] {
			continue
		}

		scopes = append(scopes, models.ResourceScope{PageContext: models.PageContext{ProjectKey: item.ProjectKey}})

		checked[item.ProjectKey] = true
	}

	return s.Platform.authorizeScopes(ctx, authorization, scopes)
}

func (s *AgentService) readSavedOperation(ctx context.Context, _ struct{}) (any, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		return nil, fmt.Errorf("运行上下文缺失")
	}

	var r *models.Run
	var err error
	if e.RegenerateFrom != nil {
		r, err = s.Store.GetRun(ctx, e.User, e.RegenerateFrom.ID, e.Cutoff)
	} else {
		if e.RootID == "" || e.RootID == e.Run.ID {
			return nil, fmt.Errorf("请明确要继续的 skill_execution_id")
		}

		r, err = s.Store.SavedSkillRun(ctx, e.User, e.Run.ConversationID, e.RootID, e.Cutoff)
	}

	if err != nil {
		return nil, err
	}

	if err := s.authorizeRegeneration(ctx, e.User, e.Authorization, r, e.Cutoff); err != nil {
		return nil, err
	}

	if e.RegenerateFrom == nil {
		if _, err := s.skillExecution(ctx, r.SkillID); err != nil {
			return nil, err
		}
		var saved models.OperationPlan
		if json.Unmarshal(r.Plan, &saved) != nil {
			return nil, common.ErrRequestConflict
		}
		e.SkillInput = saved.Input
		if r.SkillID == "project_onboarding" || r.SkillID == "service_onboarding" {
			e.OnboardingDraft = saved.Draft
		}

		metadata, err := s.Store.ContextScopes(ctx, e.User, r.ConversationID, e.Cutoff, []string{r.ID})
		if err != nil {
			return nil, err
		}

		for _, source := range metadata {
			for _, scope := range source.Scopes {
				if scope.SystemAction != "" {
					err = s.Store.RecordSystemScope(ctx, e.User, e.Run.ID, scope.SystemAction)
					e.Run.Context = models.PageContext{PageType: "efficiency"}
				} else if scope.Catalog != "" {
					err = s.Store.RecordCatalogScope(ctx, e.User, e.Run.ID, scope.Catalog)
				} else {
					err = s.recordScope(ctx, e, scope.PageContext)
				}

				if err != nil {
					return nil, err
				}
			}
		}

		if err := s.Store.RecordCatalogScope(ctx, e.User, e.Run.ID, "saved_operation"); err != nil {
			return nil, err
		}

		e.Outcome = "awaiting_input"
	} else {
		e.EvidenceRead.Store(true)
	}

	value, err := safeJSON(common.JSON(map[string]any{
		"run_id":      r.ID,
		"status":      r.Status,
		"plan_status": r.PlanStatus,
		"plan":        r.Plan,
		"read_only":   true,
	}))
	if err != nil {
		return nil, err
	}

	return value, nil
}

func regenerationTool(name, skill string) bool {
	switch skill {
	case "workflow_diagnosis":
		return name == "workflow_diagnosis" || name == "workflow_log_window"
	case "service_diagnosis":
		return name == "service_diagnosis"
	case "environment_inspection":
		return name == "environment_inspection" || name == "environment_inspection_history" || name == "environment_service_evidence"
	case "efficiency_diagnosis":
		return name == "efficiency_diagnosis"
	case "project_onboarding", "service_onboarding":
		return name == "read_saved_operation"
	}

	return false
}
