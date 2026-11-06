package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/types"
)

// 只在同次复查内复用查询结果，不能缓存到后续模型调用。
func (p *PlatformClient) authorizeScopes(ctx context.Context, authorization string, scopes []models.ResourceScope) error {
	if len(scopes) == 0 {
		return nil
	}
	uid, err := p.Authenticate(ctx, authorization)
	if err != nil {
		return &PlatformError{Status: http.StatusUnauthorized}
	}
	raw, err := p.business(ctx, authorization, http.MethodGet, "/api/v1/authorization/auth-info", url.Values{"uid": {uid}}, nil)
	if err != nil {
		return err
	}
	var resources userclient.AuthorizedResources
	if err := json.Unmarshal(raw, &resources); err != nil {
		return err
	}
	checked := map[string]bool{}
	for _, scope := range scopes {
		if scope.SystemAction != "" {
			if scope.SystemAction != efficiencyInsightAction {
				return common.ErrRequestConflict
			}
			if !resources.IsSystemAdmin && (resources.SystemActions == nil || resources.SystemActions.DataCenter == nil || !resources.SystemActions.DataCenter.ViewInsight) {
				return &PlatformError{Status: http.StatusForbidden}
			}
		}
		if scope.ProjectKey == "" && scope.WorkflowName == "" && scope.EnvName == "" {
			continue
		}
		if err := checkNames(scope.ProjectKey); err != nil {
			return err
		}
		if scope.WorkflowName == "" && scope.EnvName == "" {
			if !resources.IsSystemAdmin && resources.ProjectAuthInfo[scope.ProjectKey] == nil {
				return &PlatformError{Status: http.StatusForbidden}
			}
			continue
		}
		name, resource, action := scope.WorkflowName, types.ResourceTypeWorkflow, types.WorkflowActionView
		project := resources.ProjectAuthInfo[scope.ProjectKey]
		allowed := project != nil && project.Workflow != nil && project.Workflow.View
		if name == "" {
			name, resource, action = scope.EnvName, types.ResourceTypeEnvironment, types.EnvActionView
			allowed = project != nil && project.Env != nil && project.Env.View
			if scope.Production {
				action = types.ProductionEnvActionView
				allowed = project != nil && project.ProductionEnv != nil && project.ProductionEnv.View
			}
		}
		if err := checkNames(name); err != nil {
			return err
		}
		if resources.IsSystemAdmin {
			continue
		}
		if project == nil {
			return &PlatformError{Status: http.StatusForbidden}
		}
		if project.IsProjectAdmin || allowed {
			continue
		}
		query := url.Values{"uid": {uid}, "project_key": {scope.ProjectKey}, "resource": {resource}, "resource_name": {name}, "action": {action}}
		key := query.Encode()
		if checked[key] {
			continue
		}
		raw, err := p.business(ctx, authorization, http.MethodGet, "/api/v1/authorization/collaboration-permission", query, nil)
		if err != nil {
			return err
		}
		var result types.CheckCollaborationModePermissionResp
		if err := json.Unmarshal(raw, &result); err != nil {
			return err
		}
		if result.Error != "" || !result.HasPermission {
			return &PlatformError{Status: http.StatusForbidden}
		}
		checked[key] = true
	}
	return nil
}

func (s *AgentService) authorizeModelContext(ctx context.Context, e *execution) error {
	if e.Run.ID == "" {
		return nil
	}
	ids := append([]string{e.Run.ID}, e.HistoryRunIDs...)
	metadata, err := s.Store.ContextScopes(ctx, e.User, e.Run.ConversationID, e.Cutoff, ids)
	if err != nil {
		return err
	}
	if len(metadata) != len(ids) {
		return common.ErrNotFound
	}
	scopes := []models.ResourceScope{}
	for _, item := range metadata {
		for _, scope := range item.Scopes {
			if scope.Catalog == "" {
				scopes = append(scopes, scope)
			}
		}
	}
	return s.Platform.authorizeScopes(ctx, e.Authorization, scopes)
}
