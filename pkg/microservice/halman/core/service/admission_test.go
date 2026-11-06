package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestAdmissionWaitsUntilSlotIsReleased(t *testing.T) {
	s := &AgentService{
		maxConcurrentRuns:    1,
		admissionWaitTimeout: time.Second,
		active:               map[string]*activeRun{"occupied": {}},
	}
	done := make(chan error, 1)
	go func() {
		err := s.admit(context.Background())
		if err == nil {
			s.admissionMu.Unlock()
		}

		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("满载时应短暂等待: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	s.release("occupied")
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("释放名额后应接收请求: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("释放名额后仍未接收请求")
	}
}

func TestAdmissionCancellationAndShutdown(t *testing.T) {
	s := &AgentService{maxConcurrentRuns: 1, admissionWaitTimeout: time.Second}
	// 即使入场锁被占用，取消也不能等到锁释放才生效。
	s.admissionMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := s.admit(ctx)
	s.admissionMu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("等待中必须响应请求取消: %v", err)
	}

	s.closing = true
	if err := s.admit(context.Background()); !errors.Is(err, common.ErrCapacity) {
		t.Fatalf("关闭时不能接收新任务: %v", err)
	}

	s.closing = false
	serviceCtx, stop := context.WithCancel(context.Background())
	stop()
	s.Context = serviceCtx
	if err := s.admit(context.Background()); !errors.Is(err, common.ErrCapacity) {
		t.Fatalf("服务退出时即使有名额也不能接收新任务: %v", err)
	}
}

func TestAdmissionTimeoutDoesNotCreateRun(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	s := &AgentService{
		Store:                store,
		maxConcurrentRuns:    1,
		admissionWaitTimeout: 30 * time.Millisecond,
		active:               map[string]*activeRun{"occupied": {}},
	}
	started := time.Now()
	r, err := s.Start(
		context.Background(),
		"user",
		"conversation",
		"",
		models.RunRequest{RequestID: "new", Message: "你好"},
		&types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true}},
	)
	if r != nil || !errors.Is(err, common.ErrCapacity) || time.Since(started) < 20*time.Millisecond {
		t.Fatalf("等待超时后应友好拒绝，不创建运行或启动模型: %+v %v", r, err)
	}
}

func TestDuplicateAcceptedWhileWaitingReturnsExistingRun(t *testing.T) {
	store, mock := mockStore(t)
	req := models.RunRequest{RequestID: "request", Message: "你好"}
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}).
		AddRow("run", common.RequestFingerprint("conversation", req), now))
	mock.ExpectQuery("SELECT r.id,.*JOIN ai_conversation").WillReturnRows(runRows("run", now))
	s := &AgentService{Store: store, maxConcurrentRuns: 1, active: map[string]*activeRun{"occupied": {}}}
	r, err := s.Start(
		context.Background(),
		"user",
		"conversation",
		"",
		req,
		&types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90}},
	)
	if err != nil || r == nil || r.ID != "run" {
		t.Fatalf("等待期间被接收的重复请求应返回原运行，不报繁忙: %+v %v", r, err)
	}
}

func TestZeroWaitStillAcceptsAvailableSlot(t *testing.T) {
	s := &AgentService{maxConcurrentRuns: 1}
	if err := s.admit(context.Background()); err != nil {
		t.Fatal(err)
	}

	s.admissionMu.Unlock()
	s.active = map[string]*activeRun{"occupied": {}}
	if err := s.admit(context.Background()); !errors.Is(err, common.ErrCapacity) {
		t.Fatalf("零等待且满载时应立即返回: %v", err)
	}
}

func TestConcurrentAdmissionsRespectConfiguredLimit(t *testing.T) {
	s := &AgentService{
		maxConcurrentRuns:    3,
		admissionWaitTimeout: 5 * time.Second,
		active:               make(map[string]*activeRun),
	}
	var wg sync.WaitGroup
	peak := 0
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := s.admit(context.Background()); err != nil {
				t.Error(err)
				return
			}

			s.mu.Lock()
			s.active[id] = &activeRun{}
			peak = max(peak, len(s.active))
			s.mu.Unlock()
			s.admissionMu.Unlock()
			time.Sleep(10 * time.Millisecond)
			s.release(id)
		}(strconv.Itoa(i))
	}

	wg.Wait()
	if peak > 3 || len(s.active) != 0 {
		t.Fatalf("并发不能超限，运行结束后必须释放名额: 峰值 %d，剩余 %d", peak, len(s.active))
	}
}
