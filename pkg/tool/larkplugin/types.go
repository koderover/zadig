/*
 * Copyright 2025 The KodeRover Authors.
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

package larkplugin

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type GetTokenResponse struct {
	Data  interface{} `json:"data"`
	Error *Error      `json:"error"`
}

type PluginAccessTokenData struct {
	Token      string `json:"token"`
	ExpireTime int64  `json:"expire_time"`
}

type UserAccessTokenData struct {
	Token             string `json:"token"`
	ExpireTime        int64  `json:"expire_time"`
	RefreshToken      string `json:"refresh_token"`
	RefreshExpireTime int64  `json:"refresh_expire_time"`
	SaasTenantKey     string `json:"saas_tenant_key"`
	UserKey           string `json:"user_key"`
}

type RefreshAccessTokenData struct {
	Token             string `json:"token"`
	ExpireTime        int64  `json:"expire_time"`
	RefreshToken      string `json:"refresh_token"`
	RefreshExpireTime int64  `json:"refresh_expire_time"`
}
