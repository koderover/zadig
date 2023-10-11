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
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	redisClient *redis.Client
}

var redisClient *redis.Client

func NewRedisCache() *RedisCache {
	if redisClient == nil {
		redisClient = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
			DB:   0,
		})
	}
	return &RedisCache{redisClient: redisClient}
}

func (c *RedisCache) Write(key, val string, ttl time.Duration) error {
	_, err := c.redisClient.Set(context.TODO(), key, val, ttl).Result()
	return err
}

func (c *RedisCache) Exists(key string) (bool, error) {
	exists, err := c.redisClient.Exists(context.TODO(), key).Result()
	if err != nil {
		return false, err
	}

	if exists == 1 {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *RedisCache) GetString(key string) (string, error) {
	return c.redisClient.Get(context.TODO(), key).Result()
}
