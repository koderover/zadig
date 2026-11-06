package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *AgentService) restoreOnboardingDraft(ctx context.Context, e *execution) error {
	if e.OnboardingDraft != nil || e.RootID == "" || e.RootID == e.Run.ID {
		return nil
	}
	r, err := s.Store.SavedSkillRun(ctx, e.User, e.Run.ConversationID, e.RootID, e.Cutoff)
	if err != nil {
		return err
	}
	if r.SkillID != e.SkillID {
		return common.ErrRequestConflict
	}
	if err := s.authorizeRegeneration(ctx, e.User, e.Authorization, r, e.Cutoff); err != nil {
		return err
	}
	var plan models.OperationPlan
	if json.Unmarshal(r.Plan, &plan) != nil || plan.Draft == nil {
		return fmt.Errorf("原清单没有可恢复的初始化参数，请重新提供完整服务列表")
	}
	e.OnboardingDraft = plan.Draft
	return nil
}

func mergeOnboardingDraft(base *models.OnboardingInput, patch models.OnboardingInput) (models.OnboardingInput, error) {
	fields := patch.ProvidedFields()
	if _, provided := fields["expected_service_count"]; provided && (patch.ExpectedServiceCount < 1 || patch.ExpectedServiceCount > 20) {
		return patch, fmt.Errorf("服务总数必须为一到二十")
	}
	current := map[string]json.RawMessage{}
	if base != nil {
		if err := json.Unmarshal(common.JSON(base), &current); err != nil {
			return patch, err
		}
	}
	var services []map[string]json.RawMessage
	if err := json.Unmarshal(current["services"], &services); len(current["services"]) > 0 && err != nil {
		return patch, err
	}
	for _, name := range patch.RemoveServices {
		found := -1
		for i, item := range services {
			if draftString(item, "name") == name && name != "" {
				found = i
				break
			}
		}
		if found < 0 {
			return patch, fmt.Errorf("要移除的服务 %s 不在当前草稿中", name)
		}
		services = append(services[:found], services[found+1:]...)
	}
	if raw, provided := fields["services"]; provided {
		var changes []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &changes); err != nil {
			return patch, err
		}
		var err error
		services, err = mergeNamedInputs(services, changes, true)
		if err != nil {
			return patch, err
		}
	}
	current["services"] = common.JSON(services)
	delete(fields, "services")
	delete(fields, "remove_services")
	if raw, provided := fields["builds"]; provided {
		var before, changes []map[string]json.RawMessage
		if err := json.Unmarshal(current["builds"], &before); len(current["builds"]) > 0 && err != nil {
			return patch, err
		}
		if err := json.Unmarshal(raw, &changes); err != nil {
			return patch, err
		}
		if len(changes) > 0 {
			merged, err := mergeNamedInputs(before, changes, false)
			if err != nil {
				return patch, err
			}
			fields["builds"] = common.JSON(merged)
		}
	}
	mergeDraftObject(current, fields)
	var result models.OnboardingInput
	if err := json.Unmarshal(common.JSON(current), &result); err != nil {
		return patch, err
	}
	if _, explicit := fields["expected_service_count"]; !explicit {
		count := 0
		if base != nil {
			count = base.ExpectedServiceCount
		}
		if count == 0 {
			count = len(result.Services)
		}
		result.ExpectedServiceCount = count
	}
	result.RemoveServices = nil
	return result, nil
}

func mergeNamedInputs(before, changes []map[string]json.RawMessage, service bool) ([]map[string]json.RawMessage, error) {
	seen := map[string]bool{}
	used := map[int]bool{}
	for _, change := range changes {
		if change == nil {
			return nil, fmt.Errorf("服务或构建条目不能为空")
		}
		name := draftString(change, "name")
		if name != "" && seen[name] {
			return nil, fmt.Errorf("同批名称 %s 重复", name)
		}
		seen[name] = true
		index := -1
		for i, item := range before {
			if name != "" && draftString(item, "name") == name {
				index = i
				break
			}
		}
		if index < 0 {
			for i, item := range before {
				if !used[i] && draftString(item, "name") == "" {
					index = i
					break
				}
			}
		}
		if index < 0 {
			before = append(before, change)
			used[len(before)-1] = true
			continue
		}
		item := before[index]
		used[index] = true
		if _, provided := change["source"]; service && provided && draftString(change, "source") != draftString(item, "source") {
			for _, key := range []string{"git", "template_id", "variables", "chart_repo", "chart_name", "chart_version", "values_git", "template_fingerprint"} {
				delete(item, key)
			}
		}
		if _, provided := change["template_id"]; provided && draftString(change, "template_id") != draftString(item, "template_id") {
			delete(item, "variables")
			delete(item, "template_fingerprint")
		}
		mergeDraftObject(item, change)
	}
	return before, nil
}

func draftString(item map[string]json.RawMessage, key string) string {
	var value string
	json.Unmarshal(item[key], &value)
	return value
}

func mergeDraftObject(base, patch map[string]json.RawMessage) {
	for key, raw := range patch {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			delete(base, key)
			continue
		}
		var old, changes map[string]json.RawMessage
		if json.Unmarshal(base[key], &old) == nil && old != nil && json.Unmarshal(raw, &changes) == nil && changes != nil {
			mergeDraftObject(old, changes)
			base[key] = common.JSON(old)
		} else {
			base[key] = raw
		}
	}
}
