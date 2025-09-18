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

package larkplugin

import (
	"net/http"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	sdk "github.com/larksuite/project-oapi-sdk-golang"
	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
)

type Client struct {
	*sdk.Client
	*sdk.ClientV2
}

func NewClient(pluginID, pluginSecret, larkType string) *Client {
	return &Client{
		Client: sdk.NewClient(pluginID, pluginSecret,
			sdk.WithReqTimeout(3*time.Second),
			sdk.WithEnableTokenCache(true),
			sdk.WithHttpClient(http.DefaultClient),
			sdk.WithOpenBaseUrl(GetLarkPluginBaseUrl(larkType)),
			sdk.WithAccessTokenType(sdkcore.AccessTokenType(config.LarkPluginAccessTokenType())),
		),
		ClientV2: sdk.NewClientV2(pluginID, pluginSecret,
			sdk.WithReqTimeout(3*time.Second),
			sdk.WithEnableTokenCache(true),
			sdk.WithHttpClient(http.DefaultClient),
			sdk.WithOpenBaseUrl(GetLarkPluginBaseUrl(larkType)),
			sdk.WithAccessTokenType(sdkcore.AccessTokenType(config.LarkPluginAccessTokenType())),
		),
	}
}
