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

	"github.com/koderover/zadig/v2/pkg/config"
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

func (c *RedisCache) HWrite(key, field, val string, ttl time.Duration) error {
	_, err := c.redisClient.HSet(context.TODO(), key, field, val).Result()
	if err != nil {
		return err
	}

	// not thread safe
	if ttl > 0 {
		_, err = c.redisClient.Expire(context.Background(), key, ttl).Result()
	}
	return err
}

func (c *RedisCache) SetNX(key, val string, ttl time.Duration) error {
	_, err := c.redisClient.SetNX(context.TODO(), key, val, ttl).Result()
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

func (c *RedisCache) HGetString(key, field string) (string, error) {
	return c.redisClient.HGet(context.TODO(), key, field).Result()
}

func (c *RedisCache) HEXISTS(key, field string) (bool, error) {
	exists, err := c.redisClient.HExists(context.TODO(), key, field).Result()
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (c *RedisCache) HGetAllString(key string) (map[string]string, error) {
	return c.redisClient.HGetAll(context.Background(), key).Result()
}

func (c *RedisCache) Delete(key string) error {
	return c.redisClient.Del(context.TODO(), key).Err()
}

func (c *RedisCache) HDelete(key, field string) error {
	_, err := c.redisClient.HDel(context.Background(), key, field).Result()
	return err
}

func (c *RedisCache) Publish(channel, message string) error {
	return c.redisClient.Publish(context.Background(), channel, message).Err()
}

func (c *RedisCache) Subscribe(channel string) (<-chan *redis.Message, func() error) {
	sub := c.redisClient.Subscribe(context.Background(), channel)
	return sub.Channel(), sub.Close
}

func (c *RedisCache) FlushDBAsync() error {
	return c.redisClient.FlushDBAsync(context.Background()).Err()
}

func (c *RedisCache) ListSetMembers(key string) ([]string, error) {
	return c.redisClient.SMembers(context.Background(), key).Result()
}

func (c *RedisCache) AddElementsToSet(key string, elements []string, ttl time.Duration) error {
	if len(elements) == 0 {
		return nil
	}
	err := c.redisClient.SAdd(context.Background(), key, elements).Err()
	if err != nil {
		return err
	}
	c.redisClient.Expire(context.Background(), key, ttl)
	return nil
}

func (c *RedisCache) RemoveElementsFromSet(key string, elements []string) error {
	if len(elements) == 0 {
		return nil
	}
	return c.redisClient.SRem(context.Background(), key, elements).Err()
}

type RedisCacheAI struct {
	redisClient *redis.Client
	ttl         time.Duration
	noCache     bool
	hashKey     string
}

func NewRedisCacheAI(db int, noCache bool) ICache {
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
	return &RedisCacheAI{
		redisClient: redisClient,
		hashKey:     "zadig-ai",
		noCache:     noCache,
		ttl:         time.Minute * 60 * 24,
	}
}

func (c *RedisCacheAI) Store(key, data string) error {
	_, err := c.redisClient.HSet(context.TODO(), c.hashKey, key, data).Result()
	if err != nil {
		return err
	}

	// not thread safe
	if c.ttl > 0 {
		_, err = c.redisClient.Expire(context.Background(), c.hashKey, c.ttl).Result()
	}
	return err
}

func (c *RedisCacheAI) Load(key string) (string, error) {
	return c.redisClient.HGet(context.TODO(), c.hashKey, key).Result()
}

func (c *RedisCacheAI) List() ([]string, error) {
	result, err := c.redisClient.HGetAll(context.TODO(), c.hashKey).Result()
	if err != nil {
		return nil, err
	}

	resp := []string{}
	for _, r := range result {
		resp = append(resp, r)
	}
	return resp, nil
}

func (c *RedisCacheAI) Exists(key string) bool {
	existed, err := c.redisClient.HExists(context.TODO(), c.hashKey, key).Result()
	if err != nil {
		return false

	}
	return existed
}

func (c *RedisCacheAI) IsCacheDisabled() bool {
	return c.noCache
}
