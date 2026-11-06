package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	aguitypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
	"github.com/gin-gonic/gin"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/translator"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func aguiRequest(input aguitypes.RunAgentInput) (models.RunRequest, error) {
	if strings.TrimSpace(input.ThreadID) == "" || len(input.ThreadID) > 64 || len(input.Messages) == 0 {
		return models.RunRequest{}, fmt.Errorf("必须提供有效的 threadId 和用户消息")
	}
	if len(input.Tools) != 0 || len(input.Resume) != 0 {
		return models.RunRequest{}, fmt.Errorf("暂不支持客户端工具或 resume；操作确认请使用 confirmation 接口")
	}
	last := input.Messages[len(input.Messages)-1]
	text, plainText := last.Content.(string)
	if strings.TrimSpace(last.ID) == "" || len(last.ID) > 128 || last.Role != aguitypes.RoleUser || !plainText || len(last.ToolCalls) != 0 || last.ToolCallID != "" {
		return models.RunRequest{}, fmt.Errorf("messages 最后一项必须是带 id 的纯文本用户消息，不接受工具调用结果")
	}
	var props models.AGUIForwardedProps
	raw, err := json.Marshal(input.ForwardedProps)
	if err != nil || json.Unmarshal(raw, &props) != nil {
		return models.RunRequest{}, fmt.Errorf("forwardedProps 格式不正确")
	}
	request := models.RunRequest{
		RequestID: input.RunID, Message: text,
		Context: props.PageContext, SkillExecutionID: props.SkillExecutionID,
	}
	if err := common.ValidateRequest(request); err != nil {
		return request, fmt.Errorf("runId 对应请求标识：%w", err)
	}
	return request, nil
}

// agui godoc
// @Summary 通过 AG-UI 提交 AI 助手问题并接收事件
// @Description 使用 RunAgentInput 的 camelCase 字段。threadId 为已创建的本人会话 ID，必须与路径 id 一致，否则返回 HTTP 400/code 400；runId 为 1～64 位字母、数字、横线或下划线，用作幂等请求标识；相同 runId 的问题和页面上下文必须一致。请求体最多 1 MiB；只使用 messages 最后一项的纯文本 user 消息，消息需包含 id，最多 16000 字节；历史和工具以服务端保存的内容为准，不接受客户端工具或 resume。
// @Description 模型返回可见推理内容时发送 REASONING_START、REASONING_MESSAGE_START/CONTENT/END、REASONING_END；无推理内容时不发送。仅开启事件展示，不调整模型推理模式或强度；历史消息中的 reasoning_content 用于恢复已保存的推理内容。
// @Description forwardedProps.page_context 传递本轮页面候选，forwardedProps.skill_execution_id 关联补问；state、context 和客户端历史不作为资源授权或模型上下文。服务端先建立订阅再异步执行；断线不取消运行，重复请求不会再次执行。
// @Description SSE 仅输出 AG-UI JSON 事件，先 RUN_STARTED、STATE_SNAPSHOT，再输出文本、状态及 CUSTOM；正常以 RUN_FINISHED 或 RUN_ERROR 结束，慢连接或阶段变化通过 CUSTOM halman.resync_required 通知恢复订阅后断开。RUN_STARTED.runId 对应请求 runId，STATE_SNAPSHOT.snapshot.run.id 是后续 REST 操作和恢复订阅使用的运行 ID。待补问或待确认的阶段结束不代表 Skill 成功。
// @Tags AI助手
// @Accept json
// @Produce text/event-stream
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "会话 ID，必须与请求体 threadId 一致"
// @Param body body aguitypes.RunAgentInput true "AG-UI 问题及本轮页面候选上下文"
// @Success 200 {string} string "SSE data 为 AG-UI 事件对象；心跳为 SSE 注释"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "登录态无效或已失效"
// @Failure 403 {object} e.ErrorResponse "当前用户没有相应资源权限"
// @Failure 404 {object} e.ErrorResponse "会话或关联的业务执行不可用"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/conversations/{id}/runs [post]
func (a *API) agui(c *gin.Context) {
	var input aguitypes.RunAgentInput
	decoder := json.NewDecoder(c.Request.Body)
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "AG-UI 请求格式不正确或超过大小限制"))
		return
	}
	if input.ThreadID != c.Param("id") {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, "路径会话 ID 必须与 threadId 一致"))
		return
	}
	request, err := aguiRequest(input)
	if err != nil {
		respondError(c, e.NewWithDesc(e.ErrInvalidParam, err.Error()))
		return
	}
	subscription, err := a.Agent.StartStream(c.Request.Context(), requestUser(c), c.Param("id"), c.GetHeader("Authorization"), request, requestConfig(c))
	if err != nil {
		respondError(c, err)
		return
	}
	defer subscription.Unsubscribe()
	a.stream(c, subscription.Run.ID, subscription.Events, input.RunID)
}

// events godoc
// @Summary 通过 AG-UI 恢复订阅已有运行
// @Description 只订阅本人已有运行，不启动或重跑任务。先订阅再读取快照，SSE 只输出 AG-UI 事件；每 15 秒以注释保活并复查登录态，每个运行最多保留两个订阅，超出时淘汰最早的连接并向其发送 halman.resync_required。重连前通过历史消息接口恢复已保存内容，不重放错过的 token。
// @Description 同时返回正文与模型可见推理（REASONING_*）事件；历史消息中的 reasoning_content 用于恢复已保存的推理内容。
// @Description 生成阶段的协议 runId 为原请求标识；确认执行阶段为原请求标识加 :plan:计划版本，REST 操作始终使用 snapshot.run.id。运行状态或阶段已改变、慢连接溢出时发送 CUSTOM halman.resync_required 后断开，客户端重新查询快照并恢复订阅，不把断流视为任务结束。
// @Tags AI助手
// @Produce text/event-stream
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param id path string true "运行快照中的 id"
// @Success 200 {string} string "SSE data 为 AG-UI 事件对象；断线不取消运行"
// @Failure 400 {object} e.ErrorResponse "参数错误 code=400、许可证无效 code=7050，或业务错误 code=7213～7221"
// @Failure 401 {object} e.ErrorResponse "建立流前登录态无效或已失效"
// @Failure 404 {object} e.ErrorResponse "运行不存在、不属于本人、已删除或已到期"
// @Failure 500 {object} e.ErrorResponse "企业版许可证查询失败"
// @Router /api/halman/runs/{id}/events [get]
func (a *API) events(c *gin.Context) {
	id := c.Param("id")
	if _, err := a.Agent.Store.GetRunReference(c.Request.Context(), requestUser(c), id, cutoff(c)); err != nil {
		respondError(c, err)
		return
	}
	ch, unsubscribe, err := a.Agent.Subscribe(id)
	if err != nil {
		respondError(c, err)
		return
	}
	defer unsubscribe()
	a.stream(c, id, ch, "")
}

func (a *API) stream(c *gin.Context, id string, ch <-chan service.StreamEvent, runID string) {
	current, err := a.Agent.Store.GetRun(c.Request.Context(), requestUser(c), id, cutoff(c))
	if err != nil {
		respondError(c, err)
		return
	}
	phaseID := aguiRunID(current)
	if runID == "" {
		runID = phaseID
	}
	native, err := translator.New(c.Request.Context(), current.ConversationID, runID, translator.WithReasoningContentEnabled(true))
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	stream := &aguiStream{context: c, run: current, runID: runID, phaseID: phaseID, translator: native, writer: sse.NewSSEWriter()}
	if !stream.write(aguievents.NewRunStartedEvent(current.ConversationID, stream.runID)) ||
		!stream.snapshot(current) {
		return
	}
	if ch == nil {
		stream.finish(current)
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				latest, err := a.Agent.Store.GetRun(c.Request.Context(), requestUser(c), id, cutoff(c))
				if err != nil {
					stream.fail("读取运行状态失败，请重新查询运行状态")
				} else if stream.endMessages() && stream.snapshot(latest) {
					stream.finish(latest)
				}
				return
			}
			if !stream.event(event) {
				return
			}
		case <-ticker.C:
			if _, err := a.Agent.Platform.Authenticate(c.Request.Context(), c.GetHeader("Authorization")); err != nil {
				stream.fail(e.NewWithDesc(e.ErrUnauthorized, "登录态已失效"))
				return
			}
			if _, err := c.Writer.WriteString(": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func aguiRunID(run *models.Run) string {
	if run.PlanStatus != nil {
		switch *run.PlanStatus {
		case "executing", "succeeded", "failed", "unknown":
			return run.RequestID + ":plan:" + strconv.Itoa(run.PlanVersion)
		case "cancelled":
			var plan models.OperationPlan
			if json.Unmarshal(run.Plan, &plan) == nil {
				for _, item := range plan.Items {
					if item.Status != "" && item.Status != "pending" {
						return run.RequestID + ":plan:" + strconv.Itoa(run.PlanVersion)
					}
				}
			}
		}
	}
	return run.RequestID
}

type aguiStream struct {
	context    *gin.Context
	run        *models.Run
	runID      string
	phaseID    string
	translator translator.Translator
	writer     *sse.SSEWriter
}

func (s *aguiStream) write(event aguievents.Event) bool {
	return s.writer.WriteEvent(s.context.Request.Context(), s.context.Writer, event) == nil
}

func (s *aguiStream) snapshot(run *models.Run) bool {
	s.run = run
	return s.write(aguievents.NewStateSnapshotEvent(gin.H{"run": run}))
}

func (s *aguiStream) writeMessages(events []aguievents.Event) bool {
	// 仅公开正文与模型返回的可见推理，工具原始结果和参数不直接推送。
	for _, event := range events {
		switch event.Type() {
		case aguievents.EventTypeTextMessageStart, aguievents.EventTypeTextMessageContent, aguievents.EventTypeTextMessageEnd,
			aguievents.EventTypeReasoningStart, aguievents.EventTypeReasoningMessageStart, aguievents.EventTypeReasoningMessageContent,
			aguievents.EventTypeReasoningMessageEnd, aguievents.EventTypeReasoningEnd:
			if !s.write(event) {
				return false
			}
		}
	}
	return true
}

func (s *aguiStream) endMessages() bool {
	finalizer, ok := s.translator.(translator.PostRunFinalizingTranslator)
	if !ok {
		return true
	}
	events, err := finalizer.PostRunFinalizationEvents(s.context.Request.Context())
	return err == nil && s.writeMessages(events)
}

func (s *aguiStream) event(update service.StreamEvent) bool {
	switch update.Type {
	case "model":
		native, ok := update.Data.(*event.Event)
		if !ok || native.Response == nil {
			break
		}
		response := native.Response
		if response.IsToolResultResponse() || (len(response.Choices) > 0 &&
			((!response.IsPartial && response.Choices[0].Message.Role != model.RoleAssistant) ||
				(response.IsPartial && response.Choices[0].Delta.Role != "" && response.Choices[0].Delta.Role != model.RoleAssistant))) {
			return true
		}
		events, err := s.translator.Translate(s.context.Request.Context(), native)
		if err != nil {
			break
		}
		return s.writeMessages(events)
	case "conversation_title", "confirmation_required":
		return s.write(aguievents.NewCustomEvent("halman."+update.Type, aguievents.WithValue(update.Data)))
	case "operation":
		item, ok := update.Data.(models.OperationItem)
		var plan models.OperationPlan
		if !ok || json.Unmarshal(s.run.Plan, &plan) != nil {
			s.fail("操作状态同步失败，请重新查询运行状态")
			return false
		}
		for index, existing := range plan.Items {
			if existing.ID == item.ID {
				return s.write(aguievents.NewStateDeltaEvent([]aguievents.JSONPatchOperation{{Op: "replace", Path: "/run/plan/items/" + strconv.Itoa(index), Value: item}}))
			}
		}
	case "error":
		s.fail(update.Data)
		return false
	case "resync_required":
		s.resync()
		return false
	}
	s.fail("事件状态同步失败，请重新查询运行状态")
	return false
}

func aguiError(data any) *aguievents.RunErrorEvent {
	responseErr := e.NewWithDesc(e.ErrAIAssistantRequest, "运行处理失败，请查询运行状态")
	switch value := data.(type) {
	case string:
		responseErr = e.NewWithDesc(e.ErrAIAssistantRequest, value)
	case error:
		var httpError *e.HTTPError
		if errors.As(value, &httpError) {
			responseErr = httpError
		}
	}
	_, body := e.ErrorMessage(responseErr)
	message, _ := body["description"].(string)
	if message == "" {
		message, _ = body["message"].(string)
	}
	return aguievents.NewRunErrorEvent(message, aguievents.WithErrorCode(fmt.Sprint(body["code"])))
}

func (s *aguiStream) fail(data any) {
	if s.endMessages() {
		s.write(aguiError(data))
	}
}

func (s *aguiStream) finish(run *models.Run) {
	if !s.endMessages() {
		return
	}
	if !common.Terminal(run.Status) || aguiRunID(run) != s.phaseID {
		s.resync()
		return
	}
	if run.Status == "failed" || run.Status == "interrupted" || run.Status == "expired" {
		message := run.Error
		if message == "" {
			message = "运行未能完成，请查看运行状态"
		}
		s.fail(message)
		return
	}
	s.write(aguievents.NewRunFinishedEventWithOptions(run.ConversationID, s.runID, aguievents.WithResult(run)))
}

func (s *aguiStream) resync() {
	if s.endMessages() {
		s.write(aguievents.NewCustomEvent("halman.resync_required", aguievents.WithValue(gin.H{"run_id": s.run.ID})))
	}
}
