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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

const (
	liveFrameChannelPrefix     = "terminal_audit:live:"
	liveTerminateChannelPrefix = "terminal_audit:terminate:"
	liveStateKeyPrefix         = "terminal_audit:state:"
	liveStateTTL               = 30 * time.Second
	liveHeartbeatInterval      = 10 * time.Second
	livePublishBufferSize      = 512
)

const (
	liveMessageFrame     = "frame"
	liveMessageEnd       = "end"
	liveMessageHeartbeat = "heartbeat"
	liveMessageTerminate = "terminate"
)

type liveState struct {
	Header string `json:"header"`
	Resize string `json:"resize,omitempty"`
}

type liveMessage struct {
	Type  string `json:"type"`
	Frame string `json:"frame,omitempty"`
}

type liveSubscription interface {
	Messages() <-chan string
	Close() error
}

type liveTransport interface {
	Publish(channel, message string) (int64, error)
	Subscribe(ctx context.Context, channel string) (liveSubscription, error)
	SaveState(key string, state liveState) error
	LoadState(key string) (liveState, error)
	DeleteState(key string) error
}

type redisLiveTransport struct {
	cache *cache.RedisCache
}

func newRedisLiveTransport() liveTransport {
	return &redisLiveTransport{
		cache: cache.NewRedisCache(config.RedisCommonCacheTokenDB()),
	}
}

func (t *redisLiveTransport) Publish(channel, message string) (int64, error) {
	return t.cache.PublishCount(channel, message)
}

func (t *redisLiveTransport) Subscribe(ctx context.Context, channel string) (liveSubscription, error) {
	messages, closeSubscription, err := t.cache.SubscribeContext(ctx, channel)
	if err != nil {
		return nil, err
	}
	subscription := &redisLiveSubscription{
		messages: make(chan string, livePublishBufferSize),
		closeFn:  closeSubscription,
	}
	go func() {
		defer close(subscription.messages)
		defer subscription.Close()
		for {
			select {
			case message, ok := <-messages:
				if !ok {
					return
				}
				select {
				case subscription.messages <- message.Payload:
				default:
					return
				}
			}
		}
	}()
	return subscription, nil
}

func (t *redisLiveTransport) SaveState(key string, state liveState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return t.cache.Write(key, string(data), liveStateTTL)
}

func (t *redisLiveTransport) LoadState(key string) (liveState, error) {
	data, err := t.cache.GetString(key)
	if err != nil {
		return liveState{}, err
	}
	state := liveState{}
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return liveState{}, err
	}
	return state, nil
}

func (t *redisLiveTransport) DeleteState(key string) error {
	return t.cache.Delete(key)
}

type redisLiveSubscription struct {
	messages  chan string
	closeFn   func() error
	closeOnce sync.Once
}

func (s *redisLiveSubscription) Messages() <-chan string {
	return s.messages
}

func (s *redisLiveSubscription) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.closeFn != nil {
			err = s.closeFn()
		}
	})
	return err
}

var (
	liveTransportMu      sync.RWMutex
	liveTransportFactory = func() liveTransport {
		return newRedisLiveTransport()
	}
)

func currentLiveTransport() liveTransport {
	liveTransportMu.RLock()
	factory := liveTransportFactory
	liveTransportMu.RUnlock()
	return factory()
}

func setLiveTransportForTest(transport liveTransport) func() {
	liveTransportMu.Lock()
	previous := liveTransportFactory
	liveTransportFactory = func() liveTransport {
		return transport
	}
	liveTransportMu.Unlock()
	return func() {
		liveTransportMu.Lock()
		liveTransportFactory = previous
		liveTransportMu.Unlock()
	}
}

func liveFrameChannel(sessionID string) string {
	return liveFrameChannelPrefix + sessionID
}

func liveTerminateChannel(sessionID string) string {
	return liveTerminateChannelPrefix + sessionID
}

func liveStateKey(sessionID string) string {
	return liveStateKeyPrefix + sessionID
}

func encodeLiveMessage(message liveMessage) (string, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeLiveMessage(payload string) (liveMessage, error) {
	message := liveMessage{}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return liveMessage{}, err
	}
	return message, nil
}

type livePublisher struct {
	transport liveTransport
	sessionID string
	events    chan livePublishEvent
	done      chan struct{}
	closeOnce sync.Once
	enqueueMu sync.Mutex
	closed    bool
	stateMu   sync.Mutex
	state     liveState
}

type livePublishEvent struct {
	code  string
	frame string
	end   bool
}

func newLivePublisher(sessionID string, transport liveTransport) *livePublisher {
	publisher := &livePublisher{
		transport: transport,
		sessionID: sessionID,
		events:    make(chan livePublishEvent, livePublishBufferSize),
		done:      make(chan struct{}),
	}
	go publisher.run()
	return publisher
}

func (p *livePublisher) setHeader(header string) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.state.Header = header
	return p.transport.SaveState(liveStateKey(p.sessionID), p.state)
}

func (p *livePublisher) publish(code, frame string) {
	p.enqueueMu.Lock()
	defer p.enqueueMu.Unlock()
	if p.closed {
		return
	}
	select {
	case p.events <- livePublishEvent{code: code, frame: frame}:
	default:
		// Live observers are best effort. The recorder and object-storage cast
		// must not be slowed down by a Redis outage or a slow observer.
	}
}

func (p *livePublisher) run() {
	defer close(p.done)
	ticker := time.NewTicker(liveHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case event := <-p.events:
			if event.end {
				p.finish()
				return
			}
			if event.code == "r" {
				p.stateMu.Lock()
				p.state.Resize = event.frame
				_ = p.transport.SaveState(liveStateKey(p.sessionID), p.state)
				p.stateMu.Unlock()
			}
			payload, err := encodeLiveMessage(liveMessage{Type: liveMessageFrame, Frame: event.frame})
			if err != nil {
				continue
			}
			_, _ = p.transport.Publish(liveFrameChannel(p.sessionID), payload)
		case <-ticker.C:
			p.stateMu.Lock()
			if p.state.Header != "" {
				_ = p.transport.SaveState(liveStateKey(p.sessionID), p.state)
			}
			p.stateMu.Unlock()
			heartbeat, err := encodeLiveMessage(liveMessage{Type: liveMessageHeartbeat})
			if err == nil {
				_, _ = p.transport.Publish(liveFrameChannel(p.sessionID), heartbeat)
			}
		}
	}
}

func (p *livePublisher) finish() {
	end, err := encodeLiveMessage(liveMessage{Type: liveMessageEnd})
	if err == nil {
		_, _ = p.transport.Publish(liveFrameChannel(p.sessionID), end)
	}
	_ = p.transport.DeleteState(liveStateKey(p.sessionID))
}

func (p *livePublisher) close() {
	p.closeOnce.Do(func() {
		p.enqueueMu.Lock()
		p.closed = true
		p.events <- livePublishEvent{end: true}
		p.enqueueMu.Unlock()
		<-p.done
	})
}

func subscribeToLiveFrames(sessionID string) (<-chan string, func(), error) {
	transport := currentLiveTransport()
	ctx, cancel := context.WithCancel(context.Background())
	subscription, err := transport.Subscribe(ctx, liveFrameChannel(sessionID))
	if err != nil {
		cancel()
		return nil, nil, err
	}
	state, err := transport.LoadState(liveStateKey(sessionID))
	if err != nil {
		_ = subscription.Close()
		cancel()
		return nil, nil, fmt.Errorf("load live terminal state: %w", err)
	}
	if state.Header == "" {
		_ = subscription.Close()
		cancel()
		return nil, nil, fmt.Errorf("live terminal state has no asciicast header")
	}

	frames := make(chan string, livePublishBufferSize)
	frames <- state.Header
	if state.Resize != "" {
		frames <- state.Resize
	}
	done := make(chan struct{})
	var closeOnce sync.Once
	closeSubscription := func() {
		closeOnce.Do(func() {
			close(done)
			cancel()
			_ = subscription.Close()
		})
	}
	go func() {
		relayLiveMessages(subscription, frames, done, closeSubscription, liveStateTTL)
	}()
	return frames, closeSubscription, nil
}

func relayLiveMessages(
	subscription liveSubscription,
	frames chan string,
	done <-chan struct{},
	closeSubscription func(),
	timeout time.Duration,
) {
	defer close(frames)
	defer closeSubscription()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-done:
			return
		case <-timer.C:
			return
		case payload, ok := <-subscription.Messages():
			if !ok {
				return
			}
			message, err := decodeLiveMessage(payload)
			if err != nil {
				continue
			}
			if message.Type == liveMessageEnd {
				return
			}
			if message.Type != liveMessageFrame && message.Type != liveMessageHeartbeat {
				continue
			}
			resetTimer(timer, timeout)
			if message.Type == liveMessageHeartbeat || message.Frame == "" {
				continue
			}
			select {
			case frames <- message.Frame:
			default:
				return
			}
		}
	}
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func publishRemoteTermination(sessionID string) (int64, error) {
	return currentLiveTransport().Publish(liveTerminateChannel(sessionID), liveMessageTerminate)
}

func subscribeToTermination(ctx context.Context, sessionID string) (liveSubscription, error) {
	return currentLiveTransport().Subscribe(ctx, liveTerminateChannel(sessionID))
}

var _ liveSubscription = (*redisLiveSubscription)(nil)
