package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func (s *AgentService) Confirm(ctx context.Context, user, authorization, id string, version int, cutoff time.Time) (*models.Run, error) {
	if version < 1 {
		return nil, common.ErrRequestConflict
	}

	r, err := s.Store.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, err
	}

	if r.PlanVersion != version {
		return nil, common.ErrRequestConflict
	}

	if planAccepted(r) {
		return r, nil
	}

	var plan models.OperationPlan
	if json.Unmarshal(r.Plan, &plan) != nil || len(plan.Items) == 0 {
		return nil, common.ErrRequestConflict
	}

	e := &execution{
		Run:           r,
		User:          user,
		Authorization: authorization,
		Cutoff:        cutoff,
		SkillID:       r.SkillID,
	}
	if r.SkillExecutionID != nil {
		e.RootID = *r.SkillExecutionID
	}

	cfg, err := s.Platform.Runtime(ctx)
	if err != nil {
		return nil, err
	}

	e.Language = language.Normalize(cfg.Language)
	if !cfg.Enabled {
		return nil, common.ErrAIDisabled
	}
	if err := s.preflightPermissions(ctx, e, pendingPlan(plan)); err != nil {
		return nil, err
	}

	if err := s.verifyPlan(ctx, e, pendingPlan(plan)); err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrRequestConflict, err.Error())
	}

	if err := s.admit(ctx); err != nil {
		if errors.Is(err, common.ErrCapacity) {
			if current, lookupErr := s.Store.GetRun(ctx, user, id, cutoff); lookupErr == nil && current.PlanVersion == version && planAccepted(current) {
				return current, nil
			}
		}

		return nil, err
	}

	defer s.admissionMu.Unlock()
	s.mu.Lock()
	previousActive := s.active[id] != nil
	s.mu.Unlock()
	if previousActive {
		return nil, common.ErrBusy
	}

	r, plan, start, err := s.Store.ConfirmPlan(ctx, user, id, version, cutoff)
	if err != nil || !start {
		return r, err
	}

	e.Run = r
	runCtx, cancel := context.WithTimeout(s.Context, 10*time.Minute)
	runCtx = context.WithValue(runCtx, executionKey{}, e)
	s.mu.Lock()
	s.active[id] = &activeRun{cancel: cancel, subscribers: make(map[chan StreamEvent]struct{})}
	s.mu.Unlock()
	s.wg.Add(1)
	go s.executePlan(runCtx, cancel, e, plan)
	return r, nil
}

func planAccepted(r *models.Run) bool {
	if r.PlanStatus == nil {
		return false
	}

	switch *r.PlanStatus {
	case "executing", "succeeded", "failed", "unknown":
		return true
	}

	return false
}

func (s *AgentService) verifyPlan(ctx context.Context, e *execution, plan models.OperationPlan) error {
	ctx = withReadPhase(ctx)
	for _, item := range plan.Items {
		var id, expected, source string
		switch item.Action {
		case "create_service":
			var args models.ServiceInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			id, expected, source = args.TemplateID, args.TemplateFingerprint, args.Source
		case "restart_service", "rollback_environment":
			var args models.RepairInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			if err := s.checkRepairSnapshot(ctx, e, args); err != nil {
				return err
			}
		case "create_build":
			var args models.BuildInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			id, expected, source = args.TemplateID, args.TemplateFingerprint, "build_template"
		}

		if id != "" {
			if _, err := s.readTemplate(ctx, e, item.ProjectKey, source, id, &expected); err != nil {
				return err
			}
		}
	}

	return nil
}

func dependenciesReady(item models.OperationItem, statuses map[string]string) bool {
	for _, id := range item.DependsOn {
		if statuses[id] != "succeeded" {
			return false
		}
	}

	return true
}

func (s *AgentService) executePlan(ctx context.Context, cancel context.CancelFunc, e *execution, plan models.OperationPlan) {
	defer s.wg.Done()
	defer cancel()
	defer s.release(e.Run.ID)
	started := time.Now()
	statuses := make(map[string]string)
	overall := "succeeded"
	for i, item := range plan.Items {
		if item.Status == "succeeded" {
			statuses[item.ID] = "succeeded"
			continue
		}

		if ctx.Err() != nil {
			item.Status = "cancelled"
			item.Error = e.text("未提交的动作已停止", "Unsubmitted operations have been stopped")
		} else if !dependenciesReady(item, statuses) {
			item.Status = "blocked"
			item.Error = e.text("前置动作未成功，不提交当前动作", "A prerequisite did not succeed; this operation was not submitted")
		} else {
			cfg, err := s.Platform.Runtime(ctx)
			if err == nil && !cfg.Enabled {
				err = common.ErrAIDisabled
			}

			if err == nil {
				err = s.preflightPermissions(ctx, e, models.OperationPlan{Items: []models.OperationItem{item}})
			}

			var raw json.RawMessage
			submitted := false
			if err == nil {
				raw, err = s.submitOperation(ctx, e, item)
				submitted = err == nil
			}

			if err == nil {
				err = checkOperationResponse(item, raw)
			}

			if err == nil {
				err = s.awaitOperation(ctx, e, item)
			}

			if err == nil {
				item.Status = "succeeded"
				item.Result = common.JSON(map[string]string{"message": e.text("已完成；详细资源请在 Zadig 页面查看", "Completed; view resource details in Zadig")})
			} else {
				item.Status = operationFailureStatus(err, submitted)
				item.Error = planError(err, e.Language)
			}
		}

		if item.Status == "unknown" {
			overall = "unknown"
		} else if item.Status != "succeeded" && overall != "unknown" {
			overall = "failed"
		}

		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.Store.SaveOperation(finishCtx, e.User, e.Run.ID, i, item)
		finishCancel()
		if err != nil {
			s.publish(e.Run.ID, StreamEvent{Type: "error", Data: e.text("动作结果保存失败，停止后续操作；请核实已提交资源", "Failed to save an operation result. Remaining operations stopped; verify submitted resources")})
			recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.Store.FinishPlan(recoveryCtx, e.User, e.Run, e.RootID, "unknown", time.Since(started).Milliseconds())
			recoveryCancel()
			return
		}

		statuses[item.ID] = item.Status
		s.publish(e.Run.ID, StreamEvent{Type: "operation", Data: item.Redacted()})
	}

	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err := s.Store.FinishPlan(finishCtx, e.User, e.Run, e.RootID, overall, time.Since(started).Milliseconds()); err != nil {
		s.publish(e.Run.ID, StreamEvent{Type: "error", Data: e.text("计划结果保存失败，请核实资源状态", "Failed to save the plan result; verify resource status")})
	}
}

func operationFailureStatus(err error, submitted bool) string {
	var platform *PlatformError
	// 已提交后的取消、失权或查询失败都不能证明业务写入未发生。
	if submitted || (errors.As(err, &platform) && platform.Uncertain) {
		return "unknown"
	}

	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}

	return "failed"
}

func checkOperationResponse(item models.OperationItem, raw []byte) error {
	if item.Action != "create_service" {
		return nil
	}

	var result struct {
		Failed      []any    `json:"failedServices"`
		FailedSnake []any    `json:"failed_services"`
		Success     []string `json:"successServices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return &PlatformError{Uncertain: true}
	}

	var args models.ServiceInput
	if json.Unmarshal(item.Params, &args) != nil {
		return &PlatformError{Uncertain: true}
	}

	if strings.HasPrefix(args.Source, "helm_") && (len(result.Success) != 1 || result.Success[0] != args.Name) {
		return &PlatformError{Uncertain: true}
	}

	if len(result.Failed) > 0 || len(result.FailedSnake) > 0 {
		return &PlatformError{Uncertain: true}
	}

	return nil
}

func (s *AgentService) awaitOperation(ctx context.Context, e *execution, item models.OperationItem) error {
	if item.Action != "create_environment" && item.Action != "restart_service" && item.Action != "rollback_environment" {
		return nil
	}
	// 异步部署需要查询终态；未能证明完成时保留待核实，不能计为完成任务。
	if item.Action != "create_environment" {
		return s.awaitRepair(ctx, e, item)
	}

	var args models.EnvironmentInput
	if err := json.Unmarshal(item.Params, &args); err != nil {
		return err
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		raw, err := s.Platform.business(
			ctx,
			e.Authorization,
			http.MethodGet,
			"/api/environment/environments/"+args.Name,
			url.Values{"projectName": {item.ProjectKey}, "production": {"false"}},
			nil,
		)
		if err == nil {
			var result struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(raw, &result) == nil {
				switch result.Status {
				case "success", "Running":
					return nil
				case "failed":
					return fmt.Errorf("环境部署失败，请查看环境状态")
				}
			}
		}

		select {
		case <-ctx.Done():
			return &PlatformError{Uncertain: true}
		case <-ticker.C:
		}
	}
}

func (s *AgentService) release(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active := s.active[id]; active != nil {
		for ch := range active.subscribers {
			close(ch)
		}

		delete(s.active, id)
	}
}

func (s *AgentService) repairReady(ctx context.Context, e *execution, item models.OperationItem) (bool, error) {
	var args models.RepairInput
	if err := json.Unmarshal(item.Params, &args); err != nil {
		return false, err
	}

	pods, err := s.Platform.servicePods(ctx, e.Authorization, args.Target)
	if err != nil {
		return false, err
	}

	if pods.Truncated || len(pods.Pods) == 0 {
		return false, nil
	}

	for _, pod := range pods.Pods {
		if !pod.Ready || !pod.ContainersReady {
			return false, nil
		}
	}

	if item.Action == "restart_service" {
		old := map[string]bool{}
		for _, name := range args.PodNames {
			old[name] = true
		}

		for _, pod := range pods.Pods {
			if old[pod.Name] {
				return false, nil
			}
		}

		return len(pods.Pods) >= len(args.PodNames), nil
	}

	versions, err := s.Platform.versions(ctx, e.Authorization, args)
	if err != nil {
		return false, err
	}

	var latest environmentVersion
	for _, version := range versions {
		if version.Revision > latest.Revision {
			latest = version
		}
	}

	return latest.Revision > args.CurrentRevision && latest.Detail == "AI 助手：用户确认回滚环境版本（"+e.Run.ID+"）", nil
}

func (s *AgentService) awaitRepair(ctx context.Context, e *execution, item models.OperationItem) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		ready, err := s.repairReady(ctx, e, item)
		if err == nil && ready {
			return nil
		}

		var platform *PlatformError
		if errors.As(err, &platform) && (platform.Status == 401 || platform.Status == 403) {
			return err
		}

		select {
		case <-ctx.Done():
			return &PlatformError{Uncertain: true}
		case <-ticker.C:
		}
	}
}
