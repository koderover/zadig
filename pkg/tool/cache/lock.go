/*
Copyright 2023 The KodeRover Authors.

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

package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-redsync/redsync/v4"
	goredis_v9 "github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var resync *redsync.Redsync

func init() {
	resync = redsync.New(goredis_v9.NewPool(NewRedisCache(config.RedisCommonCacheTokenDB()).redisClient))
}

type RedisLock struct {
	key   string
	mutex *redsync.Mutex
}

func NewRedisLock(key string) *RedisLock {
	return &RedisLock{
		key:   key,
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500)),
	}
}

func NewRedisLockWithExpiry(key string, expiry time.Duration) *RedisLock {
	return &RedisLock{
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}
}

func (lock *RedisLock) Lock() error {
	return lock.LockContext(context.Background())
}

func (lock *RedisLock) LockContext(ctx context.Context) error {
	err := lock.mutex.LockContext(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "lock already taken") {
			log.Errorf("failed to acquire redis lock: %s, err: %s", lock.key, err)
		}
	}
	return err
}

func (lock *RedisLock) UnlockContext(ctx context.Context) error {
	_, err := lock.mutex.UnlockContext(ctx)
	return err
}

func (lock *RedisLock) TryLock() error {
	err := lock.mutex.TryLock()
	if err != nil {
		if strings.Contains(err.Error(), "lock already taken") {
			log.Errorf("failed to try acquire redis lock: %s, err: %s", lock.key, err)
		}
	}
	return err
}

func (lock *RedisLock) Unlock() error {
	_, err := lock.mutex.Unlock()
	return err
}

// WithRedisLock renews the lease until the operation completes.
// The callback must stop using protected state when its context is cancelled.
func WithRedisLock(key string, use func(context.Context) error) (err error) {
	mutex := NewRedisLockWithExpiry(key, time.Minute)
	if err := mutex.Lock(); err != nil {
		return fmt.Errorf("failed to acquire redis lock %s: %w", key, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				ok, err := mutex.mutex.ExtendContext(renewCtx)
				if err == nil && !ok {
					err = redsync.ErrExtendFailed
				}
				stop()
				if err != nil {
					if ctx.Err() != nil {
						done <- nil
					} else {
						cancel()
						done <- fmt.Errorf("lost redis lock %s: %w", key, err)
					}
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		renewErr := <-done
		err = errors.Join(err, renewErr, mutex.Unlock())
	}()
	return errors.Join(use(ctx), ctx.Err())
}
