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
	"errors"
	"fmt"
	"sync"
	"time"

	redisv9 "github.com/redis/go-redis/v9"

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
	liveStateReadRetries       = 5
	liveStateReadRetryDelay    = 100 * time.Millisecond
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

func subscribeRedis(ctx context.Context, redis *cache.RedisCache, channel string) (*redisLiveSubscription, error) {
	messages, closeSubscription, err := redis.SubscribeContext(ctx, channel)
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
	redis     *cache.RedisCache
	sessionID string
	events    chan livePublishEvent
	stop      chan struct{}
	closeOnce sync.Once
	enqueueMu sync.Mutex
	closed    bool
	stateMu   sync.Mutex
	state     liveState
}

type livePublishEvent struct {
	code  string
	frame string
}

func newLivePublisher(sessionID string) *livePublisher {
	publisher := &livePublisher{
		redis:     cache.NewRedisCache(config.RedisCommonCacheTokenDB()),
		sessionID: sessionID,
		events:    make(chan livePublishEvent, livePublishBufferSize),
		stop:      make(chan struct{}),
	}
	go publisher.run()
	return publisher
}

func (p *livePublisher) setHeader(header string) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.state.Header = header
	return p.saveStateLocked()
}

func (p *livePublisher) saveStateLocked() error {
	data, err := json.Marshal(p.state)
	if err != nil {
		return err
	}
	return p.redis.Write(liveStateKey(p.sessionID), string(data), liveStateTTL)
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
	ticker := time.NewTicker(liveHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			for {
				select {
				case event := <-p.events:
					p.publishEvent(event)
				default:
					p.finish()
					return
				}
			}
		case event := <-p.events:
			p.publishEvent(event)
		case <-ticker.C:
			p.stateMu.Lock()
			if p.state.Header != "" {
				_ = p.saveStateLocked()
			}
			p.stateMu.Unlock()
			heartbeat, err := encodeLiveMessage(liveMessage{Type: liveMessageHeartbeat})
			if err == nil {
				_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), heartbeat)
			}
		}
	}
}

func (p *livePublisher) publishEvent(event livePublishEvent) {
	if event.code == "r" {
		p.stateMu.Lock()
		p.state.Resize = event.frame
		_ = p.saveStateLocked()
		p.stateMu.Unlock()
	}
	payload, err := encodeLiveMessage(liveMessage{Type: liveMessageFrame, Frame: event.frame})
	if err != nil {
		return
	}
	_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), payload)
}

func (p *livePublisher) finish() {
	end, err := encodeLiveMessage(liveMessage{Type: liveMessageEnd})
	if err == nil {
		_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), end)
	}
	_ = p.redis.Delete(liveStateKey(p.sessionID))
}

func (p *livePublisher) close() {
	p.closeOnce.Do(func() {
		p.enqueueMu.Lock()
		p.closed = true
		close(p.stop)
		p.enqueueMu.Unlock()
	})
}

func subscribeToLiveFrames(sessionID string) (<-chan string, func(), error) {
	redis := cache.NewRedisCache(config.RedisCommonCacheTokenDB())
	ctx, cancel := context.WithCancel(context.Background())
	subscription, err := subscribeRedis(ctx, redis, liveFrameChannel(sessionID))
	if err != nil {
		cancel()
		return nil, nil, err
	}
	var data string
	for attempt := 0; attempt < liveStateReadRetries; attempt++ {
		data, err = redis.GetString(liveStateKey(sessionID))
		if err == nil {
			break
		}
		if !errors.Is(err, redisv9.Nil) || attempt == liveStateReadRetries-1 {
			_ = subscription.Close()
			cancel()
			return nil, nil, fmt.Errorf("load live terminal state: %w", err)
		}
		time.Sleep(liveStateReadRetryDelay)
	}
	state := liveState{}
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		_ = subscription.Close()
		cancel()
		return nil, nil, fmt.Errorf("decode live terminal state: %w", err)
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
	subscription *redisLiveSubscription,
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
	return cache.NewRedisCache(config.RedisCommonCacheTokenDB()).PublishCount(liveTerminateChannel(sessionID), liveMessageTerminate)
}

func subscribeToTermination(ctx context.Context, sessionID string) (*redisLiveSubscription, error) {
	return subscribeRedis(ctx, cache.NewRedisCache(config.RedisCommonCacheTokenDB()), liveTerminateChannel(sessionID))
}
