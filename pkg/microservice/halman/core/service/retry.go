package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func retryPlan(sourceID string, original models.OperationPlan, locale string) (models.OperationPlan, error) {
	plan := original
	plan.Items = append([]models.OperationItem(nil), original.Items...)
	plan.RetryOfRunID = sourceID
	count := 0
	for i := range plan.Items {
		item := &plan.Items[i]
		switch item.Status {
		case "succeeded":
		case "failed", "blocked", "cancelled":
			item.Status = "pending"
			item.Error = ""
			item.Result = nil
			count++
		default:
			return plan, fmt.Errorf("结果尚未核实，不能生成重试清单")
		}
	}

	if count == 0 {
		return plan, fmt.Errorf("没有可重试的失败项")
	}

	plan.Notes = append(append([]string(nil), plan.Notes...), language.Normalize(locale).Text("成功项保留且不会重复执行；仅重试本清单中的失败或被阻塞项，需要再次确认", "Successful items are preserved and will not run again. Only failed or blocked items will be retried, after confirmation"))
	return plan, nil
}

func pendingPlan(plan models.OperationPlan) models.OperationPlan {
	result := plan
	result.Items = nil
	for _, item := range plan.Items {
		if item.Status == "pending" {
			result.Items = append(result.Items, item)
		}
	}

	return result
}

func (s *AgentService) Retry(ctx context.Context, user, authorization, id, requestID string, cutoff time.Time) (*models.Run, error) {
	ctx = withReadPhase(ctx)
	source, err := s.Store.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, err
	}

	req := models.RunRequest{
		RequestID: requestID,
		Message:   "重试失败项",
		Context:   source.Context,
		RetryFrom: id,
	}
	if err := common.ValidateRequest(req); err != nil {
		return nil, err
	}

	if previous, err := s.Store.FindRequest(ctx, user, source.ConversationID, req, cutoff); err == nil {
		return previous, nil
	} else if err != common.ErrNotFound {
		return nil, err
	}

	if source.Status != "failed" || source.SkillExecutionID == nil {
		return nil, common.ErrRequestConflict
	}

	var original models.OperationPlan
	if json.Unmarshal(source.Plan, &original) != nil {
		return nil, common.ErrRequestConflict
	}

	plan, err := retryPlan(id, original, string(language.FromContext(ctx)))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrRequestConflict, err)
	}

	if err := s.authorizeRegeneration(ctx, user, authorization, source, cutoff); err != nil {
		return nil, err
	}

	e := &execution{
		Language:      language.FromContext(ctx),
		Run:           source,
		User:          user,
		Authorization: authorization,
		Cutoff:        cutoff,
		RootID:        *source.SkillExecutionID,
		SkillID:       source.SkillID,
	}

	if err := s.checkRetryConflicts(ctx, e, plan); err != nil {
		return nil, err
	}

	for i := range plan.Items {
		item := &plan.Items[i]
		if item.Status != "pending" {
			continue
		}

		if item.Action == "restart_service" || item.Action == "rollback_environment" {
			var args models.RepairInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return nil, err
			}

			if err := s.repairSnapshot(ctx, e, &args); err != nil {
				return nil, err
			}

			item.Params = common.JSON(args)
		}
	}

	pending := pendingPlan(plan)
	if err := s.preflightPermissions(ctx, e, pending); err != nil {
		return nil, err
	}

	if err := s.verifyPlan(ctx, e, pending); err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrRequestConflict, err)
	}

	if len(common.JSON(plan)) > 128<<10 {
		return nil, fmt.Errorf("%w: 重试清单过大，请缩小批次", common.ErrRequestConflict)
	}

	if err := s.admit(ctx); err != nil {
		return nil, err
	}

	defer s.admissionMu.Unlock()
	// 获得锁后再次去重，避免并发重复请求生成两份清单。
	if previous, err := s.Store.FindRequest(ctx, user, source.ConversationID, req, cutoff); err == nil {
		return previous, nil
	} else if err != common.ErrNotFound {
		return nil, err
	}

	r, err := s.Store.CreateRetryPlan(ctx, user, source, req, plan, cutoff)
	if err != nil {
		return nil, err
	}

	if err := s.persistRetryNotice(ctx, user, r); err != nil {
		r.Error = language.FromContext(ctx).Text("清单已保存，但聊天记录保存失败，请保留运行 ID 查询；未执行业务操作", "The plan was saved, but chat history could not be saved. Keep the run ID to check it later. No business operations were executed.")
		return r, nil
	}

	r.AnswerAvailable = true
	return r, nil
}

func (s *AgentService) checkRetryConflicts(ctx context.Context, e *execution, plan models.OperationPlan) error {
	newProjects := map[string]bool{}
	for _, item := range plan.Items {
		if item.Status == "pending" && item.Action == "create_project" {
			newProjects[item.ProjectKey] = true
		}
	}

	for _, item := range plan.Items {
		if item.Status != "pending" || newProjects[item.ProjectKey] {
			continue
		}

		path := ""
		switch item.Action {
		case "create_service":
			var args models.ServiceInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			if err := s.preflightService(ctx, e, item.ProjectKey, args, true); err != nil {
				return err
			}
			continue
		case "create_build":
			var args models.BuildInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			path = "/api/build/build/" + args.Name
		case "create_environment":
			var args models.EnvironmentInput
			if err := json.Unmarshal(item.Params, &args); err != nil {
				return err
			}
			path = "/api/environment/environments/" + args.Name
		default:
			continue
		}

		_, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, path, url.Values{"projectName": {item.ProjectKey}}, nil)
		if err == nil {
			return fmt.Errorf("%w: 待创建资源已经存在，不能重复创建；请先核实其状态并处理已有资源", common.ErrRequestConflict)
		}

		var platform *PlatformError
		if !errors.As(err, &platform) || platform.Status != http.StatusNotFound {
			return err
		}
	}

	return nil
}

func (s *AgentService) persistRetryNotice(ctx context.Context, user string, r *models.Run) error {
	e := &execution{Run: r, User: user, Language: language.FromContext(ctx), PersistingSnapshot: true}
	ctx = context.WithValue(ctx, executionKey{}, e)
	key := session.Key{AppName: halmanconfig.AppName, UserID: user, SessionID: r.ConversationID}
	sess, err := s.Sessions.GetSession(ctx, key, session.WithEventNum(1))
	if err != nil {
		return err
	}

	if sess == nil {
		sess, err = s.Sessions.CreateSession(ctx, key, session.StateMap{})
		if err != nil {
			return err
		}
	}

	for _, message := range []model.Message{
		model.NewUserMessage(language.FromContext(ctx).Text("请为原操作的失败项生成重试清单", "Prepare a retry plan for the failed items in the original operation")),
		model.NewAssistantMessage(language.FromContext(ctx).Text("已生成失败项重试清单，请确认后执行。成功项将保留，不会重复执行。", "A retry plan is ready for confirmation. Successful items are preserved and will not run again.")),
	} {
		evt := event.NewResponseEvent(common.NewID(), string(message.Role), &model.Response{Choices: []model.Choice{{Message: message}}})
		if err := s.Sessions.AppendEvent(ctx, sess, evt); err != nil {
			return err
		}
	}

	return s.Store.MarkPreparationAnswer(ctx, user, r.ID)
}
