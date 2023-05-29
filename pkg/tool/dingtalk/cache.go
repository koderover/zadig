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

import "sync"

var (
	globalUserInfoCacheMap = map[string]map[string]*UserInfo{}
	mu                     sync.RWMutex
)

func (c *Client) getUserInfoMap() map[string]*UserInfo {
	mu.Lock()
	defer mu.Unlock()
	if globalUserInfoCacheMap[c.AppKey] == nil {
		globalUserInfoCacheMap[c.AppKey] = map[string]*UserInfo{}
	}
	return globalUserInfoCacheMap[c.AppKey]
}

func (c *Client) storeUserInfoInCache(userID string, userInfo *UserInfo) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	c.getUserInfoMap()[userID] = userInfo
}

func (c *Client) getUserInfoFromCache(userID string) *UserInfo {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()
	return c.getUserInfoMap()[userID]
}
