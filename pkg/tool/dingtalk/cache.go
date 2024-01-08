/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dingtalk

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/json"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	"github.com/koderover/zadig/v2/pkg/config"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

var (
// globalUserInfoCacheMap = map[string]map[string]*UserInfo{}
// mu                     sync.RWMutex
)

func (c *Client) cacheKey(userID string) string {
	return fmt.Sprintf("dingtalk_%s_user_%s", c.AppKey, userID)
}

//func (c *Client) getUserInfoMap() map[string]*UserInfo {
//	oncer.Do("DingDingTalkUserCache", func() {
//		cache.NewRedisCache(config.RedisCommonCacheTokenDB())
//	})
//	mu.Lock()
//	defer mu.Unlock()
//
//	if globalUserInfoCacheMap[c.AppKey] == nil {
//		globalUserInfoCacheMap[c.AppKey] = map[string]*UserInfo{}
//	}
//	return globalUserInfoCacheMap[c.AppKey]
//}

func (c *Client) storeUserInfoInCache(userID string, userInfo *UserInfo) {
	bytes, err := json.Marshal(userInfo)
	if err != nil {
		log.Errorf("marshal user info error: %v", err)
		return
	}
	err = cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Write(c.cacheKey(userID), string(bytes), 0)
	if err != nil {
		log.Errorf("write dingding user info to cache error: %v", err)
		return
	}
}

func (c *Client) getUserInfoFromCache(userID string) *UserInfo {
	value, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).GetString(c.cacheKey(userID))
	if err != nil {
		log.Infof("get dingding user info from cache error: %v", err)
		return nil
	}

	userInfo := &UserInfo{}
	err = json.Unmarshal([]byte(value), userInfo)
	if err != nil {
		log.Errorf("unmarshal user info error: %v", err)
		return nil
	}
	return userInfo
}
