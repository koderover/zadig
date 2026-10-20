/*
Copyright 2026 The KodeRover Authors.

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

package service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
)

type CodeHostArgs struct {
	models.CodeHost
	Bot json.RawMessage `json:"bot"`
}

type CodeHostResp struct {
	*models.CodeHost
	Bot *CodeHostBotResp `json:"bot,omitempty"`
}

type CodeHostBotArgs struct {
	GitLab *CodeHostGitLabBotArgs `json:"gitlab"`
	GitHub *CodeHostGitHubBotArgs `json:"github"`
}

type CodeHostGitLabBotArgs struct {
	Token string `json:"token"`
}

type CodeHostGitHubBotArgs struct {
	AppID      *int   `json:"app_id"`
	PrivateKey string `json:"private_key"`
}

type CodeHostBotResp struct {
	Configured bool                   `json:"configured"`
	GitLab     *CodeHostGitLabBotResp `json:"gitlab,omitempty"`
	GitHub     *CodeHostGitHubBotResp `json:"github,omitempty"`
	UpdatedBy  string                 `json:"updated_by,omitempty"`
	UpdatedAt  int64                  `json:"updated_at,omitempty"`
}

type CodeHostGitLabBotResp struct {
	TokenConfigured bool   `json:"token_configured"`
	BotUsername     string `json:"bot_username"`
}

type CodeHostGitHubBotResp struct {
	AppID                int    `json:"app_id"`
	PrivateKeyConfigured bool   `json:"private_key_configured"`
	BotName              string `json:"bot_name"`
}

func applyCodeHostBot(codeHost *models.CodeHost, raw json.RawMessage, userName string) error {
	if len(raw) == 0 {
		return nil
	}
	if codeHost.IntegrationLevel != setting.IntegrationLevelSystem {
		return e.ErrInvalidParam.AddDesc("仅系统级代码源支持配置机器人身份")
	}
	if codeHost.Type != setting.SourceFromGitlab && codeHost.Type != setting.SourceFromGithub {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("代码源类型 %s 不支持配置机器人身份", codeHost.Type))
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		codeHost.Bot = nil
		return nil
	}
	args := new(CodeHostBotArgs)
	if err := json.Unmarshal(raw, args); err != nil {
		return e.ErrInvalidParam.AddDesc("invalid codehost bot args")
	}
	bot := &models.CodeHostBot{UpdatedBy: userName, UpdatedAt: time.Now().Unix()}
	var err error
	switch codeHost.Type {
	case setting.SourceFromGitlab:
		if args.GitLab == nil || args.GitHub != nil {
			return e.ErrInvalidParam.AddDesc("GitLab 代码源需传入 gitlab 配置")
		}
		bot.GitLab, err = mergeGitLabBot(codeHost, args.GitLab)
	case setting.SourceFromGithub:
		if args.GitHub == nil || args.GitLab != nil {
			return e.ErrInvalidParam.AddDesc("GitHub 代码源需传入 github 配置")
		}
		bot.GitHub, err = mergeGitHubBot(codeHost, args.GitHub)
	}
	if err != nil {
		return err
	}
	codeHost.Bot = bot
	return nil
}

func mergeGitLabBot(codeHost *models.CodeHost, args *CodeHostGitLabBotArgs) (*models.CodeHostGitLabBot, error) {
	token := args.Token
	if token == "" {
		if old := codeHost.Bot.GetGitLab(); old != nil {
			token = old.Token
		}
	}
	if token == "" {
		return nil, e.ErrInvalidParam.AddDesc("首次配置需提供 token")
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: codeHost.DisableSSL}}
	if codeHost.EnableProxy {
		proxyURL, err := url.Parse(config.ProxyHTTPSAddr())
		if err != nil {
			return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitLab Token 校验失败：%s", err))
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	cli, err := gitlab.NewClient(token, gitlab.WithBaseURL(codeHost.Address), gitlab.WithHTTPClient(&http.Client{Transport: transport, Timeout: 30 * time.Second}))
	if err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitLab Token 校验失败：%s", err))
	}
	user, _, err := cli.Users.CurrentUser()
	if err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitLab Token 校验失败：%s", err))
	}
	return &models.CodeHostGitLabBot{Token: token, BotUsername: user.Username}, nil
}

func mergeGitHubBot(codeHost *models.CodeHost, args *CodeHostGitHubBotArgs) (*models.CodeHostGitHubBot, error) {
	old := codeHost.Bot.GetGitHub()
	var appID int
	switch {
	case args.AppID != nil:
		appID = *args.AppID
		if appID <= 0 {
			return nil, e.ErrInvalidParam.AddDesc("app_id 必须为正整数")
		}
	case old != nil:
		appID = old.AppID
	default:
		return nil, e.ErrInvalidParam.AddDesc("首次配置需提供 app_id")
	}

	privateKey := args.PrivateKey
	if privateKey == "" {
		switch {
		case old == nil:
			return nil, e.ErrInvalidParam.AddDesc("首次配置需提供 private_key")
		case old.AppID != appID:
			return nil, e.ErrInvalidParam.AddDesc("变更 app_id 时需提供对应的 private_key")
		default:
			privateKey = old.PrivateKey
		}
	}

	proxy := ""
	if codeHost.EnableProxy {
		proxy = config.ProxyHTTPSAddr()
	}
	app, err := githubtool.GetApp(&githubtool.Config{
		AppID:              appID,
		AppKey:             base64.StdEncoding.EncodeToString([]byte(privateKey)),
		Proxy:              proxy,
		InsecureSkipVerify: codeHost.DisableSSL,
	})
	if err != nil {
		if errors.Is(err, githubtool.ErrInvalidAppPrivateKey) {
			return nil, e.ErrInvalidParam.AddDesc("private_key 不是合法的 PEM 格式私钥")
		}
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitHub App 校验失败：%s", err))
	}
	return &models.CodeHostGitHubBot{
		AppID:      appID,
		PrivateKey: privateKey,
		BotName:    app.GetSlug() + "[bot]",
	}, nil
}

func toCodeHostResp(codeHost *models.CodeHost) *CodeHostResp {
	resp := &CodeHostResp{CodeHost: codeHost}
	if codeHost.IntegrationLevel != setting.IntegrationLevelSystem {
		return resp
	}
	switch codeHost.Type {
	case setting.SourceFromGitlab:
		resp.Bot = &CodeHostBotResp{}
		if bot := codeHost.Bot.GetGitLab(); bot != nil {
			resp.Bot.Configured = true
			resp.Bot.GitLab = &CodeHostGitLabBotResp{TokenConfigured: true, BotUsername: bot.BotUsername}
		}
	case setting.SourceFromGithub:
		resp.Bot = &CodeHostBotResp{}
		if bot := codeHost.Bot.GetGitHub(); bot != nil {
			resp.Bot.Configured = true
			resp.Bot.GitHub = &CodeHostGitHubBotResp{AppID: bot.AppID, PrivateKeyConfigured: true, BotName: bot.BotName}
		}
	default:
		return resp
	}
	if resp.Bot.Configured {
		resp.Bot.UpdatedBy = codeHost.Bot.UpdatedBy
		resp.Bot.UpdatedAt = codeHost.Bot.UpdatedAt
	}
	return resp
}
