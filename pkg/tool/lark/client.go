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
	"net/http"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type Client struct {
	*lark.Client
}

func NewClient(appID, secret, larkType string) *Client {
	return &Client{
		Client: lark.NewClient(appID, secret,
			lark.WithReqTimeout(3*time.Second),
			lark.WithEnableTokenCache(true),
			lark.WithHttpClient(&http.Client{}),
			lark.WithOpenBaseUrl(GetLarkBaseUrl(larkType)),
		),
	}
}
