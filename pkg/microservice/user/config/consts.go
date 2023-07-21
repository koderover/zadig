/*
Copyright 2021 The KodeRover Authors.

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

package config

import (
	"github.com/koderover/zadig/pkg/setting"
)

const (
	AppState           = setting.ProductName + "user"
	SystemIdentityType = "system"
	OauthIdentityType  = "oauth"
	CLICIdentityType   = "clic"
	FeiShuEmailHost    = "smtp.feishu.cn"
)

type LoginType int

const (
	AccountLoginType LoginType = 0
)

type CLICConnectorConfig struct {
	ClientID           string   `json:"clientID"`
	ClientSecret       string   `json:"clientSecret"`
	SystemURL          string   `json:"systemURL"`
	RedirectURI        string   `json:"redirectURI"`
	TokenURL           string   `json:"tokenURL"`
	AuthorizationURL   string   `json:"authorizationURL"`
	LogoutURL          string   `json:"logoutURL"`
	UserInfoURL        string   `json:"userInfoURL"`
	Scopes             []string `json:"scopes"`
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`
}
