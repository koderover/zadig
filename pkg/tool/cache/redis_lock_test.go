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
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	goredis_v9 "github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
)

func TestRedisLock(t *testing.T) {
	log.Init(&log.Config{
		Level: "debug",
	})

	redisConfig := &redis.Options{
		Addr: fmt.Sprintf("%s:%d", "127.0.0.1", 6379),
		DB:   0,
	}
	redisClient = redis.NewClient(redisConfig)
	resync = redsync.New(goredis_v9.NewPool(redisClient))

	fmt.Println("trying to  acquire lock")
	testLock := NewRedisLock("key")
	testLock.Lock()

	fmt.Println("lock acquired!")
	time.Sleep(time.Second * 5)
	testLock.Unlock()
}
