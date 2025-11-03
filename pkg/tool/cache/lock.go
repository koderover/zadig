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
		key:   key,
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}
}

func (lock *RedisLock) Lock() error {
	err := lock.mutex.Lock()
	if err != nil {
		if !strings.Contains(err.Error(), "lock already taken") {
			log.Errorf("failed to acquire redis lock: %s, err: %s", lock.key, err)
		}
	}
	return err
}

func (lock *RedisLock) TryLock() error {
	err := lock.mutex.TryLock()
	if err != nil {
		return fmt.Errorf("try lock %s err: %s", lock.key, err)
	}
	return nil
}

func (lock *RedisLock) TryLock2() error {
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
