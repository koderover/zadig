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
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
)

type RedisCache struct {
	redisClient *redis.Client
}

var redisClient *redis.Client

// NewRedisCache callers has to make sure the caller has the settings for redis in their env variables.
func NewRedisCache(db int) *RedisCache {
	if redisClient == nil {
		redisConfig := &redis.Options{
			Addr: fmt.Sprintf("%s:%d", config.RedisHost(), config.RedisPort()),
			DB:   db,
		}

		if config.RedisUserName() != "" {
			redisConfig.Username = config.RedisUserName()
		}
		if config.RedisPassword() != "" {
			redisConfig.Password = config.RedisPassword()
		}
		redisClient = redis.NewClient(redisConfig)
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

func (c *RedisCache) Delete(key string) error {
	return c.redisClient.Del(context.TODO(), key).Err()
}

func (c *RedisCache) FlushDBAsync() error {
	return c.redisClient.FlushDBAsync(context.TODO()).Err()
}
