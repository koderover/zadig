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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	liveMessageEnd       = "__terminal_audit_end__"
	liveMessageHeartbeat = "__terminal_audit_heartbeat__"
	liveMessageTerminate = "terminate"
)

func subscribeRedis(ctx context.Context, redis *cache.RedisCache, channel string) (<-chan string, func(), error) {
	source, closeRedisSubscription, err := redis.SubscribeContext(ctx, channel)
	if err != nil {
		return nil, nil, err
	}
	messages := make(chan string, livePublishBufferSize)
	var closeOnce sync.Once
	closeSubscription := func() {
		closeOnce.Do(func() { _ = closeRedisSubscription() })
	}
	go func() {
		defer close(messages)
		defer closeSubscription()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-source:
				if !ok {
					return
				}
				select {
				case messages <- message.Payload:
				default:
					// Observer is too slow; close the subscription instead of blocking terminal I/O.
					return
				}
			}
		}
	}()
	return messages, closeSubscription, nil
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

type livePublisher struct {
	redis     *cache.RedisCache
	sessionID string
	frames    chan string
	stop      chan struct{}
	closeOnce sync.Once
	enqueueMu sync.Mutex
	closed    bool
	ready     atomic.Bool
}

func newLivePublisher(sessionID string) *livePublisher {
	publisher := &livePublisher{
		redis:     cache.NewRedisCache(config.RedisCommonCacheTokenDB()),
		sessionID: sessionID,
		frames:    make(chan string, livePublishBufferSize),
		stop:      make(chan struct{}),
	}
	go publisher.run()
	return publisher
}

func (p *livePublisher) markReady() error {
	p.ready.Store(true)
	return p.redis.Write(liveStateKey(p.sessionID), "1", liveStateTTL)
}

func (p *livePublisher) publish(frame string) {
	p.enqueueMu.Lock()
	defer p.enqueueMu.Unlock()
	if p.closed {
		return
	}
	select {
	case p.frames <- frame:
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
				case frame := <-p.frames:
					p.publishFrame(frame)
				default:
					p.finish()
					return
				}
			}
		case frame := <-p.frames:
			p.publishFrame(frame)
		case <-ticker.C:
			if p.ready.Load() {
				_ = p.redis.Write(liveStateKey(p.sessionID), "1", liveStateTTL)
			}
			_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), liveMessageHeartbeat)
		}
	}
}

func (p *livePublisher) publishFrame(frame string) {
	_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), frame)
}

func (p *livePublisher) finish() {
	_, _ = p.redis.PublishCount(liveFrameChannel(p.sessionID), liveMessageEnd)
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
	messages, closeRedisSubscription, err := subscribeRedis(ctx, redis, liveFrameChannel(sessionID))
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
			closeRedisSubscription()
			cancel()
			return nil, nil, fmt.Errorf("load live terminal state: %w", err)
		}
		time.Sleep(liveStateReadRetryDelay)
	}
	if data == "" {
		closeRedisSubscription()
		cancel()
		return nil, nil, fmt.Errorf("live terminal session is not ready")
	}

	frames := make(chan string, livePublishBufferSize)
	done := make(chan struct{})
	var closeOnce sync.Once
	closeSubscription := func() {
		closeOnce.Do(func() {
			close(done)
			cancel()
			closeRedisSubscription()
		})
	}
	go func() {
		relayLiveMessages(messages, frames, done, closeSubscription, liveStateTTL)
	}()
	return frames, closeSubscription, nil
}

func relayLiveMessages(
	messages <-chan string,
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
		case payload, ok := <-messages:
			if !ok {
				return
			}
			if payload == liveMessageEnd {
				return
			}
			resetTimer(timer, timeout)
			if payload == liveMessageHeartbeat || payload == "" {
				continue
			}
			select {
			case frames <- payload:
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
