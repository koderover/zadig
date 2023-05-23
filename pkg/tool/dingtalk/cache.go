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
	userInfoCache = map[string]*UserInfo{}
	mu            sync.RWMutex
)

func StoreUserInfoCache(userID string, userInfo *UserInfo) {
	mu.Lock()
	defer mu.Unlock()
	userInfoCache[userID] = userInfo
}

func GetUserInfoCache(userID string) *UserInfo {
	mu.RLock()
	defer mu.RUnlock()
	return userInfoCache[userID]
}
