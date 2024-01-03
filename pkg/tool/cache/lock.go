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
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	"github.com/go-redsync/redsync/v4"
	goredis_v9 "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/koderover/zadig/v2/pkg/config"
)

var resync *redsync.Redsync

func init() {
	resync = redsync.New(goredis_v9.NewPool(NewRedisCache(config.RedisCommonCacheTokenDB()).redisClient))
}

type RedisLock struct {
	mutex *redsync.Mutex
}

func NewRedisLock(key string) *RedisLock {
	return &RedisLock{
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500)),
	}
}

func NewRedisLockWithExpiry(key string, expiry time.Duration) *RedisLock {
	return &RedisLock{
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}
}

func (lock *RedisLock) Lock() error {
	err := lock.mutex.Lock()
	if err != nil {
		log.Errorf("failed to acquire redis lock, err: %s", err)
	}
	return nil
}

func (lock *RedisLock) Unlock() error {
	_, err := lock.mutex.Unlock()
	return err
}
