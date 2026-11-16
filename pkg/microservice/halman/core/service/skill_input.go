package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func resolveSkillInput[T any](ctx context.Context, s *AgentService, e *execution, input T) (T, error) {
	base := e.SkillInput
	if base == nil && (e.RegenerateFrom != nil || (e.RootID != "" && e.RootID != e.Run.ID)) {
		r := e.RegenerateFrom
		var err error
		if r == nil {
			r, err = s.Store.SavedSkillRun(ctx, e.User, e.Run.ConversationID, e.RootID, e.Cutoff)
			if err != nil {
				return input, err
			}
		}
		if r.SkillID != e.SkillID {
			return input, common.ErrRequestConflict
		}
		if err := s.authorizeRegeneration(ctx, e.User, e.Authorization, r, e.Cutoff); err != nil {
			return input, err
		}
		var plan models.OperationPlan
		if len(r.Plan) > 0 && json.Unmarshal(r.Plan, &plan) != nil {
			return input, common.ErrRequestConflict
		}
		base = plan.Input
		if e.RegenerateFrom != nil && base != nil {
			var original T
			err := json.Unmarshal(base, &original)
			return original, err
		}
		// 旧运行没有参数快照时，仅恢复其已授权目标，不使用当前页面补齐。
		if base == nil {
			base = pageSkillInput(r.Context)
		}
	}
	if base == nil && e.SkillID != "efficiency_diagnosis" {
		base = pageSkillInput(e.Run.Context)
	}
	fields := map[string]json.RawMessage{}
	if len(base) > 0 {
		if err := json.Unmarshal(base, &fields); err != nil || fields == nil {
			return input, common.ErrRequestConflict
		}
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(common.JSON(input), &patch); err != nil {
		return input, err
	}
	if e.SkillID == "service_diagnosis" {
		old, next := draftString(fields, "goal"), draftString(patch, "goal")
		if (old == "restart_service" || old == "rollback_environment") && next != "" && next != old {
			return input, fmt.Errorf("本次修复目标已确定，改变目标请开始新的任务")
		}
	}
	// 更换父级目标时丢弃旧子级，避免把另一项目的任务、服务或生产属性带入。
	for _, level := range []struct {
		key      string
		children []string
	}{
		{"project_key", []string{"workflow_name", "task_id", "job_name", "env_name", "service_name", "production", "page", "page_size"}},
		{"workflow_name", []string{"task_id", "job_name"}},
		{"task_id", []string{"job_name"}},
		{"env_name", []string{"service_name", "production", "page", "page_size"}},
		{"production", []string{"service_name", "page", "page_size"}},
	} {
		if value, exists := patch[level.key]; exists && len(fields[level.key]) > 0 && string(value) != string(fields[level.key]) {
			for _, key := range level.children {
				delete(fields, key)
			}
		}
	}
	for key, value := range patch {
		fields[key] = value
	}
	var result T
	err := json.Unmarshal(common.JSON(fields), &result)
	return result, err
}

func pageSkillInput(page models.PageContext) json.RawMessage {
	var fields map[string]any
	json.Unmarshal(common.JSON(page), &fields)
	if page.EnvName != "" {
		fields["production"] = page.Production
	}
	return common.JSON(fields)
}

func (s *AgentService) saveSkillInput(ctx context.Context, e *execution, input any, missing []string) (any, error) {
	if e.RegenerateFrom != nil {
		if len(missing) > 0 {
			return nil, fmt.Errorf("原查询参数不完整，请补充：%v", missing)
		}
		return nil, nil
	}
	if e.Outcome == "pending_confirmation" {
		return nil, fmt.Errorf("本轮已有待确认清单，请在下一轮修改查询条件")
	}
	raw := common.JSON(input)
	if len(raw) > 128<<10 {
		return nil, common.ErrInvalidRequest
	}
	root := e.RootID
	if root == "" {
		root = e.Run.ID
	}
	plan := models.OperationPlan{Items: []models.OperationItem{}, Input: raw, MissingFields: missing}
	if err := s.Store.SaveDraft(ctx, e.User, e.Run, root, e.Cutoff, plan); err != nil {
		return nil, err
	}
	e.SkillInput = raw
	e.EvidenceRead.Store(false)
	e.Outcome = ""
	if len(missing) == 0 {
		return nil, nil
	}
	e.Outcome = "awaiting_input"
	return map[string]any{"status": "awaiting_input", "input": input, "missing_fields": missing, "skill_execution_id": root}, nil
}
