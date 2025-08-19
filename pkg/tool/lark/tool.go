/*
 * Copyright 2022 The KodeRover Authors.
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

package lark

import (
	"github.com/koderover/zadig/v2/pkg/setting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func getStringFromPointer(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func getBoolFromPointer(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func getPageInfo(hasMore *bool, token *string) *pageInfo {
	return &pageInfo{
		token:   getStringFromPointer(token),
		hasMore: getBoolFromPointer(hasMore),
	}
}

func GetLarkBaseUrl(larkType string) string {
	if larkType == setting.IMLark {
		return lark.FeishuBaseUrl
	} else if larkType == setting.IMLarkIntl {
		return lark.LarkBaseUrl
	}

	return ""
}
