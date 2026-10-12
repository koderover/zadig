package webhook

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/zap"
)

func TestTimedOutWebhookTasksDoNotBlockQueue(t *testing.T) {
	synctest.Test(t, testTimedOutWebhookTasksDoNotBlockQueue)
}

func testTimedOutWebhookTasksDoNotBlockQueue(t *testing.T) {
	once.Do(func() { c = &controller{queue: make(chan *task, 2), logger: zap.NewNop()} })
	ctrl := webhookController()
	originalQueue := ctrl.queue
	ctrl.queue = make(chan *task, 2)
	defer func() { ctrl.queue = originalQueue }()

	client := NewClient()
	results := make(chan error, 2)
	go func() { results <- client.AddWebHook(&TaskOption{}) }()
	go func() { results <- client.RemoveWebHook(&TaskOption{}) }()

	// Both tasks have started, but finish only after their callers time out.
	tasks := []*task{<-ctrl.queue, <-ctrl.queue}
	for range tasks {
		select {
		case err := <-results:
			if err == nil || err.Error() != "timed out waiting for the task" {
				t.Fatalf("expected timeout, got %v", err)
			}
		case <-time.After((taskTimeoutSecond + 5) * time.Second):
			t.Fatal("webhook caller did not time out")
		}
	}
	for _, task := range tasks {
		if !task.canceled.Load() {
			t.Fatal("timed out task was not canceled")
		}
		task.err = errors.New("late webhook failure")
		select {
		case task.doneCh <- struct{}{}:
		default:
			t.Fatal("completion blocks after caller timeout")
		}
	}

	go func() { results <- client.AddWebHook(&TaskOption{}) }()
	next := <-ctrl.queue
	next.err = errors.New("next webhook failure")
	next.doneCh <- struct{}{}
	select {
	case err := <-results:
		if err == nil || err.Error() != "next webhook failure" {
			t.Fatalf("next task did not return its result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("next task was blocked")
	}
}

func TestWebhookWorkerSkipsCanceledTask(t *testing.T) {
	ctrl := &controller{queue: make(chan *task, 1), logger: zap.NewNop()}
	task := &task{doneCh: make(chan struct{}, 1)}
	task.canceled.Store(true)
	ctrl.queue <- task
	if !ctrl.processNextWorkItem() || task.err != nil || len(task.doneCh) != 0 {
		t.Fatal("worker processed a canceled task")
	}
}
