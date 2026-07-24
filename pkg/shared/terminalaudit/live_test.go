/*
Copyright 2026 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package terminalaudit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type fakeLiveTransport struct {
	mu            sync.Mutex
	nextID        int
	subscriptions map[string]map[int]chan string
	states        map[string]liveState
	published     map[string][]string
	subscribeErr  error
	saveStateErr  error
}

func newFakeLiveTransport() *fakeLiveTransport {
	return &fakeLiveTransport{
		subscriptions: make(map[string]map[int]chan string),
		states:        make(map[string]liveState),
		published:     make(map[string][]string),
	}
}

func (t *fakeLiveTransport) Publish(channel, message string) (int64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.published[channel] = append(t.published[channel], message)
	var count int64
	for _, subscriber := range t.subscriptions[channel] {
		subscriber <- message
		count++
	}
	return count, nil
}

func (t *fakeLiveTransport) Subscribe(ctx context.Context, channel string) (liveSubscription, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.subscribeErr != nil {
		return nil, t.subscribeErr
	}
	id := t.nextID
	t.nextID++
	messages := make(chan string, livePublishBufferSize)
	if t.subscriptions[channel] == nil {
		t.subscriptions[channel] = make(map[int]chan string)
	}
	t.subscriptions[channel][id] = messages
	return &fakeLiveSubscription{
		messages: messages,
		closeFn: func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			if _, ok := t.subscriptions[channel][id]; ok {
				delete(t.subscriptions[channel], id)
				close(messages)
			}
			return nil
		},
	}, nil
}

func (t *fakeLiveTransport) SaveState(key string, state liveState) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.saveStateErr != nil {
		return t.saveStateErr
	}
	t.states[key] = state
	return nil
}

func (t *fakeLiveTransport) LoadState(key string) (liveState, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state, ok := t.states[key]
	if !ok {
		return liveState{}, fmt.Errorf("state not found")
	}
	return state, nil
}

func (t *fakeLiveTransport) DeleteState(key string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, key)
	return nil
}

type fakeLiveSubscription struct {
	messages <-chan string
	closeFn  func() error
}

func (s *fakeLiveSubscription) Messages() <-chan string {
	return s.messages
}

func (s *fakeLiveSubscription) Close() error {
	return s.closeFn()
}

func TestLivePublisherStreamsFramesThroughSharedTransport(t *testing.T) {
	transport := newFakeLiveTransport()
	restore := setLiveTransportForTest(transport)
	defer restore()

	publisher := newLivePublisher("session-1", transport)
	if err := publisher.setHeader(`{"version":2,"width":80,"height":24}`); err != nil {
		t.Fatalf("set header: %v", err)
	}
	frames, unsubscribe, err := subscribeToLiveFrames("session-1")
	if err != nil {
		t.Fatalf("subscribe to live frames: %v", err)
	}
	defer unsubscribe()

	if got := receiveFrame(t, frames); got != `{"version":2,"width":80,"height":24}` {
		t.Fatalf("header = %q", got)
	}

	publisher.publish("o", `[1,"o","hello"]`)
	if got := receiveFrame(t, frames); got != `[1,"o","hello"]` {
		t.Fatalf("frame = %q", got)
	}

	publisher.close()
	select {
	case _, ok := <-frames:
		if ok {
			t.Fatal("expected live frame channel to close")
		}
	case <-time.After(time.Second):
		t.Fatal("live frame channel did not close")
	}
	if _, err := transport.LoadState(liveStateKey("session-1")); err == nil {
		t.Fatal("expected live state to be deleted when publisher closes")
	}
}

func TestLivePublisherClosePublishesQueuedFramesBeforeEnd(t *testing.T) {
	transport := newFakeLiveTransport()
	publisher := newLivePublisher("session-tail", transport)

	const frameCount = 100
	for i := 0; i < frameCount; i++ {
		publisher.publish("o", fmt.Sprintf(`[%d,"o","tail"]`, i))
	}
	publisher.close()

	transport.mu.Lock()
	published := append([]string(nil), transport.published[liveFrameChannel("session-tail")]...)
	transport.mu.Unlock()
	if len(published) != frameCount+1 {
		t.Fatalf("published message count = %d, want %d", len(published), frameCount+1)
	}
	for i, payload := range published {
		message, err := decodeLiveMessage(payload)
		if err != nil {
			t.Fatalf("decode published message %d: %v", i, err)
		}
		if i < frameCount && message.Type != liveMessageFrame {
			t.Fatalf("message %d type = %q, want %q", i, message.Type, liveMessageFrame)
		}
		if i == frameCount && message.Type != liveMessageEnd {
			t.Fatalf("last message type = %q, want %q", message.Type, liveMessageEnd)
		}
	}
}

func TestLivePublisherSetHeaderReturnsStateSaveError(t *testing.T) {
	transport := newFakeLiveTransport()
	transport.saveStateErr = fmt.Errorf("redis unavailable")
	publisher := newLivePublisher("session-header-error", transport)
	defer publisher.close()

	if err := publisher.setHeader(`{"version":2}`); err == nil {
		t.Fatal("expected state save error")
	}
}

func TestSubscribeToLiveFramesRejectsStateWithoutHeader(t *testing.T) {
	transport := newFakeLiveTransport()
	if err := transport.SaveState(liveStateKey("session-without-header"), liveState{
		Resize: `[0.1,"r","80x24"]`,
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	restore := setLiveTransportForTest(transport)
	defer restore()

	if _, _, err := subscribeToLiveFrames("session-without-header"); err == nil {
		t.Fatal("expected missing asciicast header to be rejected")
	}
}

func TestRelayLiveMessagesClosesWhenHeartbeatExpires(t *testing.T) {
	messages := make(chan string)
	subscription := &fakeLiveSubscription{
		messages: messages,
		closeFn: func() error {
			close(messages)
			return nil
		},
	}
	frames := make(chan string)
	done := make(chan struct{})

	go relayLiveMessages(subscription, frames, done, func() {}, 20*time.Millisecond)

	select {
	case _, ok := <-frames:
		if ok {
			t.Fatal("expected frame channel to close after heartbeat timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("frame channel did not close after heartbeat timeout")
	}
}

func TestRemoteTerminationReachesOwningInstanceSubscription(t *testing.T) {
	transport := newFakeLiveTransport()
	restore := setLiveTransportForTest(transport)
	defer restore()

	subscription, err := subscribeToTermination(context.Background(), "session-2")
	if err != nil {
		t.Fatalf("subscribe to termination: %v", err)
	}
	defer subscription.Close()

	subscribers, err := publishRemoteTermination("session-2")
	if err != nil {
		t.Fatalf("publish termination: %v", err)
	}
	if subscribers != 1 {
		t.Fatalf("termination subscriber count = %d, want 1", subscribers)
	}
	if got := receiveFrame(t, subscription.Messages()); got != liveMessageTerminate {
		t.Fatalf("termination message = %q", got)
	}
}

func TestRegisteredSessionHandlesRemoteTermination(t *testing.T) {
	transport := newFakeLiveTransport()
	restore := setLiveTransportForTest(transport)
	defer restore()

	terminated := make(chan struct{})
	if err := RegisterActiveSession("session-3", func() {
		close(terminated)
	}); err != nil {
		t.Fatalf("register active session: %v", err)
	}
	defer UnregisterActiveSession("session-3")

	subscribers, err := publishRemoteTermination("session-3")
	if err != nil {
		t.Fatalf("publish termination: %v", err)
	}
	if subscribers != 1 {
		t.Fatalf("termination subscriber count = %d, want 1", subscribers)
	}
	select {
	case <-terminated:
	case <-time.After(time.Second):
		t.Fatal("registered session did not terminate")
	}
	if got := ResolveSessionStatus("session-3", models.TerminalSessionStatusFinished); got != models.TerminalSessionStatusAborted {
		t.Fatalf("resolved status = %q, want %q", got, models.TerminalSessionStatusAborted)
	}
}

func TestRegisterActiveSessionReturnsTerminationSubscriptionError(t *testing.T) {
	transport := newFakeLiveTransport()
	transport.subscribeErr = fmt.Errorf("redis unavailable")
	restore := setLiveTransportForTest(transport)
	defer restore()

	if err := RegisterActiveSession("session-subscribe-error", func() {}); err == nil {
		t.Fatal("expected termination subscription error")
	}
	if _, ok := registry.load("session-subscribe-error"); ok {
		t.Fatal("session with failed termination subscription must not be registered")
	}
}

func receiveFrame(t *testing.T, frames <-chan string) string {
	t.Helper()
	select {
	case frame, ok := <-frames:
		if !ok {
			t.Fatal("frame channel closed")
		}
		return frame
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for frame")
		return ""
	}
}
