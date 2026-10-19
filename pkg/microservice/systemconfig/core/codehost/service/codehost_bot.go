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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
)

type CodeHostBotArgs struct {
	GitLab *CodeHostGitLabBotArgs `json:"gitlab"`
	GitHub *CodeHostGitHubBotArgs `json:"github"`
}

type CodeHostGitLabBotArgs struct {
	Token string `json:"token"`
}

type CodeHostGitHubBotArgs struct {
	AppID      int    `json:"app_id"`
	PrivateKey string `json:"private_key"`
}

type CodeHostBotResp struct {
	CodeHostID int                    `json:"codehost_id"`
	Type       string                 `json:"type"`
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

func GetCodeHostBot(id int, _ *zap.SugaredLogger) (*CodeHostBotResp, error) {
	codeHost, err := getBotCodeHost(id)
	if err != nil {
		return nil, err
	}
	return toCodeHostBotResp(codeHost), nil
}

func UpdateCodeHostBot(id int, args *CodeHostBotArgs, userName string, log *zap.SugaredLogger) (*CodeHostBotResp, error) {
	codeHost, err := getBotCodeHost(id)
	if err != nil {
		return nil, err
	}

	bot := &models.CodeHostBot{
		UpdatedBy: userName,
		UpdatedAt: time.Now().Unix(),
	}
	switch codeHost.Type {
	case setting.SourceFromGitlab:
		if args.GitLab == nil || args.GitHub != nil {
			return nil, e.ErrInvalidParam.AddDesc("GitLab 代码源需传入 gitlab 配置")
		}
		bot.GitLab, err = mergeGitLabBot(codeHost, args.GitLab)
	case setting.SourceFromGithub:
		if args.GitHub == nil || args.GitLab != nil {
			return nil, e.ErrInvalidParam.AddDesc("GitHub 代码源需传入 github 配置")
		}
		bot.GitHub, err = mergeGitHubBot(codeHost, args.GitHub)
	}
	if err != nil {
		return nil, err
	}

	if err := mongodb.NewCodehostColl().UpdateSystemCodeHostBot(id, bot); err != nil {
		log.Errorf("failed to update bot of codehost %d: %s", id, err)
		return nil, err
	}
	codeHost.Bot = bot
	return toCodeHostBotResp(codeHost), nil
}

func DeleteCodeHostBot(id int, log *zap.SugaredLogger) error {
	if _, err := getBotCodeHost(id); err != nil {
		return err
	}
	if err := mongodb.NewCodehostColl().DeleteSystemCodeHostBot(id); err != nil {
		log.Errorf("failed to delete bot of codehost %d: %s", id, err)
		return err
	}
	return nil
}

func getBotCodeHost(id int) (*models.CodeHost, error) {
	codeHost, err := mongodb.NewCodehostColl().GetCodeHostByID(id, false)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, e.ErrNotFound.AddDesc(fmt.Sprintf("代码源 %d 不存在", id))
		}
		return nil, err
	}
	if codeHost.IntegrationLevel != setting.IntegrationLevelSystem {
		return nil, e.ErrInvalidParam.AddDesc("仅系统级代码源支持配置机器人身份")
	}
	if codeHost.Type != setting.SourceFromGitlab && codeHost.Type != setting.SourceFromGithub {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("代码源类型 %s 不支持配置机器人身份", codeHost.Type))
	}
	return codeHost, nil
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
	appID := args.AppID
	if appID == 0 && old != nil {
		appID = old.AppID
	}
	if appID <= 0 {
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

func toCodeHostBotResp(codeHost *models.CodeHost) *CodeHostBotResp {
	resp := &CodeHostBotResp{
		CodeHostID: codeHost.ID,
		Type:       codeHost.Type,
	}
	switch codeHost.Type {
	case setting.SourceFromGitlab:
		if bot := codeHost.Bot.GetGitLab(); bot != nil {
			resp.Configured = true
			resp.GitLab = &CodeHostGitLabBotResp{TokenConfigured: true, BotUsername: bot.BotUsername}
		}
	case setting.SourceFromGithub:
		if bot := codeHost.Bot.GetGitHub(); bot != nil {
			resp.Configured = true
			resp.GitHub = &CodeHostGitHubBotResp{AppID: bot.AppID, PrivateKeyConfigured: true, BotName: bot.BotName}
		}
	}
	if resp.Configured {
		resp.UpdatedBy = codeHost.Bot.UpdatedBy
		resp.UpdatedAt = codeHost.Bot.UpdatedAt
	}
	return resp
}
