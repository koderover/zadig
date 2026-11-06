package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	sessionmysql "trpc.group/trpc-go/trpc-agent-go/session/mysql"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/types"
)

type executionKey struct{}

type execution struct {
	Run                *models.Run
	User               string
	Authorization      string
	Cutoff             time.Time
	PersistenceFailed  atomic.Bool
	EvidenceRead       atomic.Bool
	SkillID            string
	Skills             []models.Skill
	ContinuingSkill    string
	Reports            map[string]reportResult
	InspectedServices  map[string]bool
	OnboardingDraft    *models.OnboardingInput
	SkillInput         json.RawMessage
	RootID             string
	RegenerateFrom     *models.Run
	Outcome            string
	PersistingSnapshot bool
	HistoryRunIDs      []string
}

// maxRunSubscribers 是单个运行同时保留的事件流连接数。
const maxRunSubscribers = 2

type StreamEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type RunSubscription struct {
	Run         *models.Run
	Events      <-chan StreamEvent
	Unsubscribe func()
}

type activeRun struct {
	cancel      context.CancelFunc
	subscribers map[chan StreamEvent]struct{}
	// order 按订阅先后记录连接，可能残留已退订项，读取时需以 subscribers 为准。
	order []chan StreamEvent
}

// evictOldest 淘汰最早的存活订阅，调用方需持有 AgentService.mu。
func (a *activeRun) evictOldest() {
	for len(a.order) > 0 {
		oldest := a.order[0]
		a.order = a.order[1:]
		if _, ok := a.subscribers[oldest]; !ok {
			continue
		}

		// 发送都在持锁下进行，腾出一个位置后写入不会阻塞。
		select {
		case <-oldest:
		default:
		}
		oldest <- StreamEvent{Type: "resync_required"}
		close(oldest)
		delete(a.subscribers, oldest)
		return
	}
}

type AgentService struct {
	Store                *ormrepo.Store
	Platform             *PlatformClient
	Sessions             session.Service
	Context              context.Context
	admissionMu          sync.Mutex
	maxConcurrentRuns    int
	admissionWaitTimeout time.Duration
	maintenanceMu        sync.Mutex
	maintenance          models.MaintenanceStatus
	closing              bool
	mu                   sync.Mutex
	active               map[string]*activeRun
	wg                   sync.WaitGroup
}

func NewAgentService(ctx context.Context, store *ormrepo.Store, platform *PlatformClient, cfg *halmanconfig.Config) (*AgentService, error) {
	s := &AgentService{
		Store:                store,
		Platform:             platform,
		Context:              ctx,
		active:               make(map[string]*activeRun),
		maxConcurrentRuns:    cfg.MaxConcurrentRuns,
		admissionWaitTimeout: cfg.AdmissionWaitTimeout,
	}
	sessions, err := sessionmysql.NewService(
		sessionmysql.WithMySQLInstance(halmanconfig.AppName),
		sessionmysql.WithSkipDBInit(true),
		sessionmysql.WithSessionEventLimit(60),
		sessionmysql.WithGetSessionHook(s.sessionContext),
		sessionmysql.WithAppendEventHook(s.beforePersist),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化消息存储失败")
	}

	s.Sessions = sessions
	return s, nil
}

func (s *AgentService) sessionContext(h *session.GetSessionContext, next func() (*session.Session, error)) (*session.Session, error) {
	state, err := next()
	if err != nil || state == nil {
		return state, err
	}

	e, ok := h.Context.Value(executionKey{}).(*execution)
	if !ok {
		return nil, fmt.Errorf("运行上下文缺失")
	}

	if e.PersistingSnapshot {
		return state, nil
	}
	// 只查询本次窗口涉及的运行，不扫描整个会话的运行历史或 JSON。
	ids := make(map[string]bool)
	runIDs := []string{}
	for _, item := range state.Events {
		if item.RequestID != "" && !ids[item.RequestID] {
			ids[item.RequestID] = true
			runIDs = append(runIDs, item.RequestID)
		}
	}

	metadata, err := s.Store.ContextScopes(h.Context, e.User, e.Run.ConversationID, e.Cutoff, runIDs)
	if err != nil {
		return nil, err
	}

	versions, err := s.Store.MessageVersions(h.Context, e.User, e.Run.ConversationID, e.Cutoff, runIDs)
	if err != nil {
		return nil, err
	}

	allowed := map[string]bool{}
	authorizationCache := map[string]bool{}
	for _, item := range metadata {
		page := item.Context
		allowed[item.ID] = !item.Scoped && page.ProjectKey == "" && page.WorkflowName == "" && page.EnvName == "" && page.ServiceName == "" &&
			page.TaskID == 0 &&
			page.JobName == ""
		if !item.Scoped {
			continue
		}

		permitted := true
		for _, scope := range item.Scopes {
			if scope.Catalog != "" || (scope.SystemAction == "" && (scope.ProjectKey == "" || !sameTarget(page, e.Run.Context))) {
				permitted = false
				break
			}

			key := string(common.JSON(scope))
			result, known := authorizationCache[key]
			if !known {
				result = s.Platform.authorizeScope(h.Context, e.Authorization, scope) == nil
				authorizationCache[key] = result
			}

			if !result {
				permitted = false
				break
			}
		}

		allowed[item.ID] = permitted
	}

	groups := make(map[string]string)
	filtered := make([]event.Event, 0, len(state.Events))
	regenerating := ""
	if e.RegenerateFrom != nil {
		regenerating = e.RegenerateFrom.ID
		if e.RegenerateFrom.QuestionSourceRunID != nil {
			regenerating = *e.RegenerateFrom.QuestionSourceRunID
		}
	}

	for _, item := range state.Events {
		version, ok := versions[item.RequestID]
		if !ok || !version.QuestionAvailable || version.OriginalID == regenerating {
			continue
		}

		groups[item.RequestID] = version.OriginalID
		if item.RequestID == version.LatestID || (!version.Regenerated && userEvent(item)) {
			filtered = append(filtered, item)
		}
	}

	state.Events = contextEvents(filtered, allowed, e.Cutoff, 64<<10, groups)
	state.Events = completeContextTurns(state.Events, groups)
	kept := map[string]bool{}
	for _, id := range e.HistoryRunIDs {
		kept[id] = true
	}
	for _, item := range state.Events {
		if item.RequestID != e.Run.ID && !kept[item.RequestID] {
			kept[item.RequestID] = true
			e.HistoryRunIDs = append(e.HistoryRunIDs, item.RequestID)
		}
	}
	// 暂不启用摘要，避免缺少来源范围和有效期的框架状态绕过历史过滤。
	state.State = session.StateMap{}
	state.Summaries = nil
	return state, nil
}

// 窗口、到期或权限过滤可能截断轮次；工具调用与结果不完整时整轮舍弃。
func completeContextTurns(events []event.Event, groups map[string]string) []event.Event {
	type turn struct {
		question bool
		invalid  bool
		pending  map[string]bool
	}
	turns := map[string]*turn{}
	for _, item := range events {
		id := groups[item.RequestID]
		state := turns[id]
		if state == nil {
			state = &turn{pending: map[string]bool{}}
			turns[id] = state
		}

		if item.Response == nil {
			continue
		}

		for _, choice := range item.Choices {
			message := choice.Message
			state.question = state.question || message.Role == model.RoleUser
			for _, call := range message.ToolCalls {
				state.invalid = state.invalid || call.ID == "" || state.pending[call.ID]
				state.pending[call.ID] = true
			}

			if message.Role == model.RoleTool {
				state.invalid = state.invalid || !state.pending[message.ToolID]
				delete(state.pending, message.ToolID)
			}
		}
	}

	result := make([]event.Event, 0, len(events))
	for _, item := range events {
		state := turns[groups[item.RequestID]]
		if state.question && !state.invalid && len(state.pending) == 0 {
			result = append(result, item)
		}
	}

	return result
}

func userEvent(item event.Event) bool {
	return item.Response != nil && len(item.Choices) == 1 && item.Choices[0].Message.Role == model.RoleUser
}

func contextEvents(events []event.Event, allowed map[string]bool, cutoff time.Time, budget int, groups map[string]string) []event.Event {
	group := func(id string) string {
		if groups[id] != "" {
			return groups[id]
		}

		return id
	}
	sizes := make(map[string]int)
	for _, item := range events {
		if allowed[item.RequestID] && !item.Timestamp.Before(cutoff) {
			raw, err := json.Marshal(item)
			if err == nil {
				sizes[group(item.RequestID)] += len(raw)
			}
		}
	}

	selected := make(map[string]bool)
	for i := len(events) - 1; i >= 0; i-- {
		id := group(events[i].RequestID)
		if selected[id] || sizes[id] == 0 {
			continue
		}

		if sizes[id] > budget {
			break
		}

		budget -= sizes[id]
		selected[id] = true
	}

	result := make([]event.Event, 0, len(events))
	for _, item := range events {
		if allowed[item.RequestID] && selected[group(item.RequestID)] && !item.Timestamp.Before(cutoff) {
			result = append(result, item)
		}
	}

	return result
}

func (s *AgentService) beforePersist(h *session.AppendEventContext, next func() error) (err error) {
	e, ok := h.Context.Value(executionKey{}).(*execution)
	if !ok {
		return fmt.Errorf("运行上下文缺失")
	}

	defer func() {
		if err != nil {
			e.PersistenceFailed.Store(true)
		}
	}()
	h.Event.RequestID = e.Run.ID
	if e.RegenerateFrom != nil && h.Event.Response != nil && len(h.Event.Choices) == 1 &&
		h.Event.Choices[0].Message.Role == model.RoleUser {
		h.Session.UpdateUserSession(h.Event)
		return nil
	}

	if h.Event.Response != nil && h.Event.Error != nil {
		h.Event.Error = &model.ResponseError{Message: "模型处理失败", Type: "model_error"}
	}

	if !h.Event.IsPartial {
		data, err := json.Marshal(h.Event)
		if err != nil {
			return err
		}

		if len(data) > 256<<10 {
			return fmt.Errorf("消息事件超过存储大小限制")
		}
	}

	return next()
}

func makeModel(cfg *types.AIAssistantRuntime, options ...openai.Option) (model.Model, error) {
	if cfg.ModelError != "" {
		return nil, fmt.Errorf("%s", cfg.ModelError)
	}

	if cfg.LLMIntegrationID == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("请先在系统设置中选择 AI 助手模型")
	}

	if cfg.Token == "" {
		return nil, fmt.Errorf("所选模型缺少访问凭据")
	}

	if cfg.Protocol != "" && cfg.Protocol != "openai" {
		return nil, fmt.Errorf("当前聊天接入仅支持 OpenAI 兼容协议，请选择兼容模型")
	}

	if cfg.Provider == "azure_openai" || cfg.Provider == "azure_ad_openai" {
		return nil, fmt.Errorf("当前聊天接入尚未支持 Azure 模型协议")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil || proxy.Host == "" {
			return nil, fmt.Errorf("模型代理配置无效")
		}

		transport.Proxy = http.ProxyURL(proxy)
	}

	opts := []openai.Option{
		openai.WithAPIKey(cfg.Token),
		openai.WithHTTPClientOptions(openai.WithHTTPClientTransport(transport), openai.WithHTTPClientTimeout(3*time.Minute)),
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		if cfg.Provider == "openai" {
			baseURL = "https://api.openai.com/v1"
		} else {
			return nil, fmt.Errorf("所选模型缺少接口地址")
		}
	}

	endpoint, err := url.Parse(baseURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil {
		return nil, fmt.Errorf("模型接口地址无效")
	}

	opts = append(opts, openai.WithBaseURL(baseURL))
	return openai.New(cfg.Model, append(opts, options...)...), nil
}

func makeRunner(cfg *types.AIAssistantRuntime, sessions session.Service, platform *PlatformClient, authorization string, services ...*AgentService) (runner.Runner, error) {
	chatModel, err := makeModel(cfg)
	if err != nil {
		return nil, err
	}

	callbacks := model.NewCallbacks().RegisterBeforeModel(func(ctx context.Context, req *model.Request) (*model.Response, error) {
		if _, err := platform.Authenticate(ctx, authorization); err != nil {
			return nil, err
		}
		current, err := platform.Runtime(ctx)
		if err != nil {
			return nil, err
		}
		if !current.Enabled {
			return nil, common.ErrAIDisabled
		}

		if e, ok := ctx.Value(executionKey{}).(*execution); ok {
			req.Messages = append([]model.Message{model.NewSystemMessage(skillInstructions(e))}, req.Messages...)
			for name := range req.Tools {
				if skill := businessToolSkill(name); skill != "" && !skillAllowed(e, skill) {
					delete(req.Tools, name)
				}
			}
			if e.RegenerateFrom != nil {
				if len(services) == 0 {
					return nil, common.ErrRequestConflict
				}

				if err := services[0].authorizeRegeneration(ctx, e.User, e.Authorization, e.RegenerateFrom, e.Cutoff); err != nil {
					return nil, err
				}

				for name := range req.Tools {
					if !regenerationTool(name, e.SkillID) {
						delete(req.Tools, name)
					}
				}

				req.Messages = append(
					[]model.Message{model.NewSystemMessage("本轮仅重新回答原问题，沿用原资源上下文；重新读取诊断证据，或调用 read_saved_operation 解释原操作快照。禁止创建、修改或确认清单，不执行或重放任何业务写入，不把原操作结果当作本轮再次执行。")},
					req.Messages...,
				)
			}

			if e.RootID != "" && e.RootID != e.Run.ID && e.RegenerateFrom == nil {
				instruction := "本轮明确关联原 Skill 执行，业务工具自动恢复参数草稿；只提交用户本轮补充或修改的字段。不要根据新页面或模糊历史覆盖原参数；需要查看草稿时可调用 read_saved_operation。收到 missing_fields 后只补问缺失项，不报告完成。"
				req.Messages = append(
					[]model.Message{model.NewSystemMessage(instruction)},
					req.Messages...,
				)
			}

			if len(services) > 0 {
				if err := services[0].authorizeModelContext(ctx, e); err != nil {
					return nil, err
				}
			}

			candidate, _ := json.Marshal(e.Run.Context)
			req.Messages = append([]model.Message{model.NewSystemMessage("本轮页面候选上下文（不是操作授权；用户明确目标优先，歧义时补问）：" + string(candidate))}, req.Messages...)
		}

		return nil, nil
	})
	agentOptions := []llmagent.Option{
		llmagent.WithModel(chatModel),
		llmagent.WithInstruction(builtinInstructions),
		llmagent.WithGenerationConfig(model.GenerationConfig{Stream: true, MaxTokens: model.IntPtr(4096)}),
		llmagent.WithMaxLLMCalls(8),
		llmagent.WithMaxToolIterations(6),
		llmagent.WithEnableParallelTools(false),
		llmagent.WithModelCallbacks(callbacks),
	}
	if len(services) > 0 {
		agentOptions = append(agentOptions, llmagent.WithTools(services[0].tools()))
	}

	ag := llmagent.New("zadig-assistant", agentOptions...)
	return runner.NewRunner(halmanconfig.AppName, ag, runner.WithSessionService(sessions)), nil
}

func (s *AgentService) Start(ctx context.Context, user, conversation, authorization string, req models.RunRequest, cfg *types.AIAssistantRuntime) (*models.Run, error) {
	subscription, err := s.start(ctx, user, conversation, authorization, req, cfg, false)
	if err != nil {
		return nil, err
	}
	return subscription.Run, nil
}

// StartStream 在启动执行前注册订阅，避免首批输出早于 HTTP 事件流建立。
func (s *AgentService) StartStream(ctx context.Context, user, conversation, authorization string, req models.RunRequest, cfg *types.AIAssistantRuntime) (*RunSubscription, error) {
	return s.start(ctx, user, conversation, authorization, req, cfg, true)
}

func (s *AgentService) subscription(record *models.Run, subscribe bool) (*RunSubscription, error) {
	result := &RunSubscription{Run: record, Unsubscribe: func() {}}
	if !subscribe {
		return result, nil
	}
	var err error
	result.Events, result.Unsubscribe, err = s.Subscribe(record.ID)
	return result, err
}

func (s *AgentService) start(ctx context.Context, user, conversation, authorization string, req models.RunRequest, cfg *types.AIAssistantRuntime, subscribe bool) (*RunSubscription, error) {
	if err := common.ValidateRequest(req); err != nil {
		return nil, err
	}

	expiresBefore := time.Now().UTC().AddDate(0, 0, -cfg.HistoryRetentionDays)
	if previous, err := s.Store.FindRequest(ctx, user, conversation, req, expiresBefore); err == nil {
		return s.subscription(previous, subscribe)
	} else if !errors.Is(err, common.ErrNotFound) {
		return nil, err
	}

	if !cfg.Enabled {
		return nil, common.ErrAIDisabled
	}
	if err := s.admit(ctx); err != nil {
		// 等待期间同一请求可能已由另一调用接收，超时前再核对一次。
		if errors.Is(err, common.ErrCapacity) {
			previous, lookupErr := s.Store.FindRequest(ctx, user, conversation, req, expiresBefore)
			if lookupErr == nil {
				return s.subscription(previous, subscribe)
			}

			if !errors.Is(lookupErr, common.ErrNotFound) {
				return nil, lookupErr
			}
		}

		return nil, err
	}

	defer s.admissionMu.Unlock()
	r, err := makeRunner(cfg, s.Sessions, s.Platform, authorization, s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrModelUnavailable, err)
	}
	skills, continuingSkill, err := s.skillSnapshot(ctx, user, conversation, req, expiresBefore)
	if err != nil {
		r.Close()
		return nil, err
	}
	// 入场只串行执行短数据库事务，不在锁内调用模型或业务 HTTP。
	record, created, err := s.Store.BeginRun(ctx, user, conversation, req, cfg.LLMIntegrationID, cfg.Model)
	if err != nil {
		r.Close()
		return nil, err
	}

	if !created {
		r.Close()
		return s.subscription(record, subscribe)
	}

	runCtx, cancel := context.WithTimeout(s.Context, 5*time.Minute)
	e := &execution{
		Run:             record,
		User:            user,
		Authorization:   authorization,
		Cutoff:          expiresBefore,
		RootID:          req.SkillExecutionID,
		RegenerateFrom:  req.RegenerateFrom,
		Skills:          skills,
		ContinuingSkill: continuingSkill,
	}
	if req.RegenerateFrom != nil {
		e.SkillID = req.RegenerateFrom.SkillID
	}

	runCtx = context.WithValue(runCtx, executionKey{}, e)
	s.mu.Lock()
	s.active[record.ID] = &activeRun{cancel: cancel, subscribers: make(map[chan StreamEvent]struct{})}
	snapshot := *record
	result := &RunSubscription{Run: &snapshot, Unsubscribe: func() {}}
	if subscribe {
		result.Events, result.Unsubscribe = s.subscribeLocked(record.ID)
	}
	s.mu.Unlock()
	s.wg.Add(1)
	go s.execute(runCtx, cancel, r, user, record, req.Message, cfg)
	return result, nil
}

func (s *AgentService) execute(ctx context.Context, cancel context.CancelFunc, r runner.Runner, user string, record *models.Run, message string, cfg *types.AIAssistantRuntime) {
	defer s.wg.Done()
	defer cancel()
	defer r.Close()
	status, failure := "succeeded", ""
	input, output := 0, 0
	hasAnswer := false
	ch, err := r.Run(ctx, user, record.ConversationID, model.NewUserMessage(message), agent.WithRequestID(record.ID))
	if err != nil {
		status, failure = "failed", "启动模型运行失败"
	} else {
		for e := range ch {
			if e.Response == nil {
				continue
			}

			if e.Error != nil {
				status, failure = "failed", "模型处理失败"
				continue
			}

			// 框架仍会更新用量计时，只复制事件转换需要的字段。
			response := (&model.Response{
				ID: e.Response.ID, Object: e.Object, IsPartial: e.IsPartial, Done: e.Done,
				Choices: e.Choices,
			}).Clone()
			// 重连可能从中间分片开始，补齐角色供框架生成有效的消息起始事件。
			if response.IsPartial {
				for i := range response.Choices {
					if response.Choices[i].Delta.Role == "" {
						response.Choices[i].Delta.Role = model.RoleAssistant
					}
				}
			}
			s.publish(record.ID, StreamEvent{Type: "model", Data: &event.Event{Response: response}})

			if !e.IsPartial && e.Usage != nil {
				input += e.Usage.PromptTokens
				output += e.Usage.CompletionTokens
			}

			for _, choice := range e.Choices {
				if !e.IsPartial && choice.Message.Role == model.RoleAssistant && len(choice.Message.ToolCalls) == 0 &&
					strings.TrimSpace(choice.Message.Content) != "" {
					hasAnswer = true
				}
			}
		}
	}

	if e, ok := ctx.Value(executionKey{}).(*execution); ok && e.PersistenceFailed.Load() {
		status, failure = "failed", "消息保存失败，请查询运行状态"
	}

	if status == "succeeded" && !hasAnswer {
		status, failure = "failed", "模型未返回有效回答"
	}

	if e, ok := ctx.Value(executionKey{}).(*execution); ok && status == "succeeded" {
		if e.Outcome != "" {
			status = e.Outcome
		} else if e.SkillID != "" && (!e.EvidenceRead.Load() || !diagnosisGoalComplete(e)) {
			status, failure = "failed", "未完成必要查询或操作，不能判定 Skill 成功"
		}
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		status, failure = "failed", "运行超时"
	} else if ctx.Err() != nil {
		status, failure = "cancelled", "运行已取消或超时"
	}

	record.AnswerAvailable = hasAnswer && status != "cancelled" && ctx.Err() == nil
	if e, ok := ctx.Value(executionKey{}).(*execution); ok && (e.PersistenceFailed.Load() || (e.RegenerateFrom != nil && status != "succeeded")) {
		record.AnswerAvailable = false
	}
	// 正文结果先确定，附加标题耗尽剩余时限时保留已完成回答；用户主动取消仍生效。
	if record.AnswerAvailable && (status == "succeeded" || status == "awaiting_input" || status == "pending_confirmation") {
		titleInput, titleOutput := s.generateConversationTitle(ctx, user, record, message, cfg)
		input += titleInput
		output += titleOutput
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		status, failure = "cancelled", "运行已取消"
		record.AnswerAvailable = false
	}
	// 本地终态事务幂等，可有限重试；不重新调用模型或提交业务操作。
	var finishErr error
	for attempt := 0; attempt < 3; attempt++ {
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 3*time.Second)
		finishErr = s.Store.FinishRun(finishCtx, user, record, status, failure, input, output)
		finishCancel()
		if finishErr == nil {
			break
		}
	}

	if finishErr != nil {
		s.publish(record.ID, StreamEvent{Type: "error", Data: "保存运行结果失败，请查询运行状态"})
	}

	s.mu.Lock()
	if active := s.active[record.ID]; active != nil {
		for subscriber := range active.subscribers {
			close(subscriber)
		}

		delete(s.active, record.ID)
	}

	s.mu.Unlock()
}

func (s *AgentService) publish(id string, e StreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active := s.active[id]; active != nil {
		for subscriber := range active.subscribers {
			select {
			case subscriber <- e:
			default:
				// 腾出一个位置通知客户端恢复快照，不能将慢连接截断误判为运行完成。
				select {
				case <-subscriber:
				default:
				}
				subscriber <- StreamEvent{Type: "resync_required"}
				close(subscriber)
				delete(active.subscribers, subscriber)
			}
		}
	}
}

func (s *AgentService) Subscribe(id string) (<-chan StreamEvent, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.active[id]
	if active == nil {
		return nil, func() {}, nil
	}

	// 名额满时保留最新连接：未释放的旧连接不能把用户挡在运行之外，被淘汰方收到恢复提示。
	if len(active.subscribers) >= maxRunSubscribers {
		active.evictOldest()
	}

	ch, unsubscribe := s.subscribeLocked(id)
	return ch, unsubscribe, nil
}

func (s *AgentService) subscribeLocked(id string) (<-chan StreamEvent, func()) {
	ch := make(chan StreamEvent, 32)
	active := s.active[id]
	live := active.order[:0]
	for _, existing := range active.order {
		if _, ok := active.subscribers[existing]; ok {
			live = append(live, existing)
		}
	}
	active.order = append(live, ch)
	active.subscribers[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if current := s.active[id]; current != nil {
			if _, ok := current.subscribers[ch]; ok {
				delete(current.subscribers, ch)
				close(ch)
			}
		}
	}
}

func (s *AgentService) Cancel(ctx context.Context, user, id string) error {
	if err := s.Store.CancelRun(ctx, user, id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if active := s.active[id]; active != nil {
		active.cancel()
	}

	return nil
}

func (s *AgentService) Close() error {
	s.admissionMu.Lock()
	s.closing = true
	s.mu.Lock()
	for _, active := range s.active {
		active.cancel()
	}

	s.mu.Unlock()
	s.admissionMu.Unlock()
	s.wg.Wait()
	return s.Sessions.Close()
}
