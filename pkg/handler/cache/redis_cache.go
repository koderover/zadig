/*
Copyright 2022 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"sync"
)

const istioCacheKey = "istio-cache-store"

type redisCacheStore struct {
	store      *sync.Map
	redisCache *cache.RedisCache
}

func NewRedisCache() Cacher {
	return &redisCacheStore{
		redisCache: cache.NewRedisCache(config.RedisCommonCacheTokenDB()),
	}
}

func (c *redisCacheStore) Get(key string) (value interface{}, exists bool) {
	value, err := c.redisCache.HGetString(istioCacheKey, key)
	if err != nil {
		return nil, false
	}
	return value, true
}

func (c *redisCacheStore) Set(key string, value interface{}) {
	c.redisCache.HWrite(istioCacheKey, key, fmt.Sprint(value), 0)
}

func (c *redisCacheStore) Delete(key string) {
	c.redisCache.HDelete(istioCacheKey, key)
}

func (c *redisCacheStore) List() map[string]interface{} {
	data, _ := c.redisCache.HGetAllString(istioCacheKey)
	ret := make(map[string]interface{})
	for k, v := range data {
		ret[k] = v
	}
	return ret
}

func (c *redisCacheStore) Purge() {
	c.redisCache.Delete(istioCacheKey)
}
