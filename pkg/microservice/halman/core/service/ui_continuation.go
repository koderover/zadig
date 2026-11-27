package service

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func (s *AgentService) prepareUIModelRequest(ctx context.Context, e *execution, req *model.Request) (*model.Response, error) {
	if e.Run.RepliedCardRunID != "" {
		for key := range req.Tools {
			if key != "read_saved_operation" {
				delete(req.Tools, key)
			}
		}
		req.Messages = append([]model.Message{model.NewSystemMessage(
			"本轮用户消息接替了一张等待输入的卡片。补充原任务时先调用 read_saved_operation，再只提交本轮明确改动。若原卡没有已保存草稿，按本轮和上一轮消息继续理解任务。仅当用户明确开始无关的新任务时，调用 read_saved_operation 并传 start_new=true，保留原草稿。意图不明确时先追问。",
		)}, req.Messages...)
	}
	if u := e.UIContinuation; u != nil {
		if err := s.authorizeRegeneration(ctx, e.User, e.Authorization, u.SourceRun, e.Cutoff); err != nil {
			return nil, err
		}
		if u.OnboardingPatch != nil {
			patch := *u.OnboardingPatch
			u.OnboardingPatch = nil
			if _, err := s.prepareProject(ctx, patch); err != nil {
				return nil, err
			}
			if e.UIPending {
				return uiAssistantResponse(e.Language.Text("请继续补充卡片中的信息。", "Please complete the next card.")), nil
			}
			if e.Outcome == "pending_confirmation" {
				return uiAssistantResponse(e.Language.Text("参数已齐全，请核对操作清单并确认。", "Review and confirm the prepared operations.")), nil
			}
		}
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == model.RoleUser {
				req.Messages = append([]model.Message(nil), req.Messages...)
				req.Messages[i].Content += "\n\n交互卡提交数据（用户输入）:\n" + string(u.Input)
				break
			}
		}
	}
	if e.UIFallback != "" {
		// The business validator supplied a single actionable clarification.
		return uiAssistantResponse(e.UIFallback), nil
	}
	if e.UIPending {
		req.Tools = nil
	}
	return nil, nil
}

func uiAssistantResponse(message string) *model.Response {
	return &model.Response{
		ID: common.NewID(), Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{Message: model.NewAssistantMessage(message)}},
	}
}

func (s *AgentService) readCardReplySource(ctx context.Context, e *execution) (*models.Run, error) {
	run, err := s.Store.GetRun(ctx, e.User, e.Run.RepliedCardRunID, e.Cutoff)
	if err != nil {
		return nil, err
	}
	if run.ConversationID != e.Run.ConversationID {
		return nil, common.ErrRequestConflict
	}
	if len(run.Plan) == 0 || run.SkillExecutionID == nil {
		return nil, fmt.Errorf("%w: 此卡片没有已保存的业务草稿，可根据用户的补充继续对话", common.ErrNotFound)
	}
	if *run.SkillExecutionID != run.ID {
		root, err := s.Store.GetRunReference(ctx, e.User, *run.SkillExecutionID, e.Cutoff)
		if err != nil {
			return nil, err
		}
		if root.ConversationID != run.ConversationID || root.SkillID != run.SkillID ||
			root.SkillExecutionID == nil || *root.SkillExecutionID != root.ID {
			return nil, common.ErrRequestConflict
		}
		run.SkillStatus = root.SkillStatus
	}
	return run, nil
}
