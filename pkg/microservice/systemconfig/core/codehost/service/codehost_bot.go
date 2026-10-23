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
	"crypto/tls"
	"encoding/base64"
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

func validateCodeHostBot(codeHost *models.CodeHost) error {
	if codeHost.AdvancedConfig == nil {
		return nil
	}
	if codeHost.IntegrationLevel != setting.IntegrationLevelSystem {
		return e.ErrInvalidParam.AddDesc("仅系统级代码源支持配置机器人身份")
	}
	switch codeHost.Type {
	case setting.SourceFromGitlab:
		bot := codeHost.AdvancedConfig.GitLabBot
		if bot == nil || codeHost.AdvancedConfig.GitHubBot != nil {
			return e.ErrInvalidParam.AddDesc("GitLab 代码源需传入 gitlab 配置")
		}
		if bot.Token == "" {
			codeHost.AdvancedConfig = nil
			return nil
		}
		username, err := getGitLabUsername(codeHost, bot.Token)
		if err != nil {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitLab Token 校验失败：%s", err))
		}
		bot.BotUsername = username
	case setting.SourceFromGithub:
		bot := codeHost.AdvancedConfig.GitHubBot
		if bot == nil || codeHost.AdvancedConfig.GitLabBot != nil {
			return e.ErrInvalidParam.AddDesc("GitHub 代码源需传入 github 配置")
		}
		if bot.AppID == 0 && bot.PrivateKey == "" {
			codeHost.AdvancedConfig = nil
			return nil
		}
		if bot.AppID <= 0 {
			return e.ErrInvalidParam.AddDesc("app_id 必须为正整数")
		}
		if bot.PrivateKey == "" {
			return e.ErrInvalidParam.AddDesc("需提供 private_key")
		}
		proxy := ""
		if codeHost.EnableProxy {
			proxy = config.ProxyHTTPSAddr()
		}
		app, err := githubtool.GetApp(&githubtool.Config{
			AppID:              bot.AppID,
			AppKey:             base64.StdEncoding.EncodeToString([]byte(bot.PrivateKey)),
			Proxy:              proxy,
			InsecureSkipVerify: codeHost.DisableSSL,
		})
		if err != nil {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("GitHub App 校验失败：%s", err))
		}
		bot.BotName = app.GetSlug() + "[bot]"
	default:
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("代码源类型 %s 不支持配置机器人身份", codeHost.Type))
	}
	return nil
}

func getGitLabUsername(codeHost *models.CodeHost, token string) (string, error) {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: codeHost.DisableSSL}}
	if codeHost.EnableProxy {
		proxyURL, err := url.Parse(config.ProxyHTTPSAddr())
		if err != nil {
			return "", err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	cli, err := gitlab.NewClient(token, gitlab.WithBaseURL(codeHost.Address), gitlab.WithHTTPClient(&http.Client{Transport: transport, Timeout: 30 * time.Second}))
	if err != nil {
		return "", err
	}
	user, _, err := cli.Users.CurrentUser()
	if err != nil {
		return "", err
	}
	return user.Username, nil
}
