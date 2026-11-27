package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func uiCardSourceKey(name string, value any) string {
	h := sha256.Sum256(common.JSON(value))
	return name + ":" + hex.EncodeToString(h[:16])
}
func (s *AgentService) emitUICard(ctx context.Context, e *execution, kind string, in models.UICardInput, key string) (*models.UICard, error) {
	if e.UIPending || e.Outcome == "pending_confirmation" {
		return nil, common.ErrRequestConflict
	}
	status := "display"
	for _, a := range in.Actions {
		if a.Type == "submit" {
			status = "preparing"
		}
	}
	if e.RegenerateFrom != nil {
		return nil, common.ErrRequestConflict
	}
	now := time.Now().UTC()
	expires := now.Add(30 * time.Minute)
	c := models.UICard{DraftFields: key == "project_onboarding", SchemaVersion: 1, ConversationID: e.Run.ConversationID, RunID: e.Run.ID, RunCreatedAt: e.Run.CreatedAt, AnswerVersion: e.Run.AnswerVersion, Type: kind, Title: in.Title, Status: status, Data: common.JSON(in.Data), Actions: in.Actions}
	if c.AnswerVersion < 1 {
		c.AnswerVersion = 1
	}
	if status == "preparing" {
		c.ExpiresAt = &expires
	}
	card, err := s.Store.UpsertUICard(ctx, e.User, c, key)
	if err != nil {
		return nil, err
	}
	e.Run.HasUICards = true
	if status == "preparing" {
		e.UIPending = true
		e.Outcome = "awaiting_input"
	}
	s.publish(e.Run.ID, StreamEvent{Type: "ui_card", Data: card})
	return card, nil
}
func uiExecution(ctx context.Context) (*execution, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		return nil, common.ErrInvalidRequest
	}
	return e, nil
}
func (s *AgentService) requestForm(ctx context.Context, in models.UICardInput) (any, error) {
	return s.presentInteractive(ctx, "form", in)
}
func (s *AgentService) requestQuestion(ctx context.Context, in models.UICardInput) (any, error) {
	return s.presentInteractive(ctx, "question", in)
}
func (s *AgentService) requestResourceSelection(ctx context.Context, in models.UICardInput) (any, error) {
	return s.presentInteractive(ctx, "resource_selector", in)
}
func (s *AgentService) presentActions(ctx context.Context, in models.UICardInput) (any, error) {
	return s.presentInteractive(ctx, "actions", in)
}
func (s *AgentService) presentInteractive(ctx context.Context, kind string, in models.UICardInput) (any, error) {
	e, err := uiExecution(ctx)
	if err != nil {
		return nil, err
	}
	if kind == "question" && len(in.Actions) == 0 {
		in.Actions = []models.UIAction{
			{ID: "submit", Label: e.Language.Text("继续", "Continue"), Type: "submit"},
		}
		if models.AllQuestionsOptional(in.Data.Questions) {
			in.Actions = append(in.Actions, models.UIAction{ID: "skip", Label: e.Language.Text("跳过", "Skip"), Type: "submit"})
		}
	} else if (kind == "form" || kind == "resource_selector") && len(in.Actions) == 0 {
		in.Actions = []models.UIAction{{ID: "submit", Label: e.Language.Text("提交并继续", "Submit and continue"), Type: "submit"}}
	}
	callID, _ := tool.ToolCallIDFromContext(ctx)
	if callID == "" {
		callID = uiCardSourceKey(kind, in)
	}
	for _, a := range in.Actions {
		if a.Type == "navigate" && a.Target != nil {
			if err := s.Platform.authorizeContext(ctx, e.Authorization, *a.Target); err != nil {
				return nil, err
			}
			if err := s.recordScope(ctx, e, *a.Target); err != nil {
				return nil, err
			}
		}
	}
	card, err := s.emitUICard(ctx, e, kind, in, "tool:"+callID)
	if err != nil {
		return nil, err
	}
	return map[string]string{"card_id": card.ID, "status": card.Status}, nil
}
func uiToolCallback(ctx context.Context, args *tool.AfterToolArgs) (*tool.AfterToolResult, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok || args.Error != nil {
		return nil, nil
	}
	if e.UIPending {
		return &tool.AfterToolResult{SkipSummarization: true}, nil
	}
	return nil, nil
}
func (s *AgentService) AuthorizedUICards(ctx context.Context, user, auth, run string, cutoff time.Time) ([]models.UICard, error) {
	cards, err := s.Store.RunUICards(ctx, user, run, cutoff)
	if err != nil {
		return nil, err
	}
	return s.FilterUICards(ctx, user, auth, cards, cutoff)
}
func (s *AgentService) FilterUICards(ctx context.Context, user, auth string, cards []models.UICard, cutoff time.Time) ([]models.UICard, error) {
	out := []models.UICard{}
	runAllowed := map[string]bool{}
	authorize := s.Platform.scopeAuthorizer(ctx, auth)
cardsLoop:
	for _, card := range cards {
		allowed, checked := runAllowed[card.RunID]
		if !checked {
			scopes, err := s.Store.ContextScopes(ctx, user, card.ConversationID, cutoff, []string{card.RunID})
			if err != nil {
				return nil, err
			}
			if len(scopes) != 1 {
				return nil, common.ErrNotFound
			}
			allowed = true
			for _, scope := range scopes[0].Scopes {
				if err := authorize(scope); err != nil {
					if !cardAccessDenied(err) {
						return nil, err
					}
					allowed = false
					break
				}
			}
			runAllowed[card.RunID] = allowed
		}
		if !allowed {
			continue
		}
		for _, value := range card.ResourceSelections {
			if common.UIValueEmpty(value) {
				continue
			}
			values, multiple := value.([]any)
			if !multiple {
				values = []any{value}
			}
			for _, selected := range values {
				var resource models.UIResource
				if json.Unmarshal(common.JSON(selected), &resource) != nil || resource.ProjectKey == "" {
					continue cardsLoop
				}
				if err := authorize(uiResourceScope(resource)); err != nil {
					if cardAccessDenied(err) {
						continue cardsLoop
					}
					return nil, err
				}
			}
		}
		out = append(out, card)
	}
	return out, nil
}

func cardAccessDenied(err error) bool {
	var platform *PlatformError
	return errors.Is(err, common.ErrNotFound) || errors.As(err, &platform) && (platform.Status == http.StatusNotFound || platform.Status == http.StatusForbidden)
}

const uiCardInstructions = "补问优先使用卡片。简单选择或短回答使用 request_question，数字、开关、长文本或多字段使用 request_form，按权限选择项目、环境或服务使用 request_resource_selection，可选的后续动作使用 present_actions。卡片仅收集业务参数，禁止索要密码、Token 或其他凭据。只问尚未确定的信息。提交等待用户，业务写入仍须经过现有确认。用户跳过选填项时不要再问同一个问题；回复中不要逐字复述表单内容。发卡失败时只用简短文字询问最先缺失的参数。"
