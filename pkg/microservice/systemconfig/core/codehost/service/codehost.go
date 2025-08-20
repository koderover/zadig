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

package service

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"

	zadigconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/internal/oauth"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

const callback = "/api/directory/codehosts/callback"

func CreateSystemCodeHost(codehost *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if codehost.Type == types.ProviderOther {
		codehost.IsReady = "2"
	}
	if codehost.Type == types.ProviderGerrit {
		codehost.IsReady = "2"
		codehost.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", codehost.Username, codehost.Password)))
	}
	if codehost.Type == types.ProviderPerforce {
		codehost.IsReady = "2"
	}

	if codehost.Alias != "" {
		if _, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(codehost.Alias); err == nil {
			return nil, fmt.Errorf("alias cannot have the same name")
		}
	}

	codehost.CreatedAt = time.Now().Unix()
	codehost.UpdatedAt = time.Now().Unix()

	list, err := mongodb.NewCodehostColl().CodeHostList()
	if err != nil {
		return nil, err
	}
	codehost.ID = len(list) + 1
	return mongodb.NewCodehostColl().AddSystemCodeHost(codehost)
}

func EncypteCodeHost(encryptedKey string, codeHosts []*models.CodeHost, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	aesKey, err := aslan.New(zadigconfig.AslanServiceAddress()).GetTextFromEncryptedKey(encryptedKey)
	if err != nil {
		log.Errorf("ListCodeHost GetTextFromEncryptedKey error:%s", err)
		return nil, err
	}
	var result []*models.CodeHost
	for _, codeHost := range codeHosts {
		if len(codeHost.Password) > 0 {
			codeHost.Password, err = crypto.AesEncryptByKey(codeHost.Password, aesKey.PlainText)
			if err != nil {
				log.Errorf("ListCodeHost AesEncryptByKey error:%s", err)
				return nil, err
			}
		}
		if len(codeHost.ClientSecret) > 0 {
			codeHost.ClientSecret, err = crypto.AesEncryptByKey(codeHost.ClientSecret, aesKey.PlainText)
			if err != nil {
				log.Errorf("ListCodeHost AesEncryptByKey error:%s", err)
				return nil, err
			}
		}
		if len(codeHost.SSHKey) > 0 {
			codeHost.SSHKey, err = crypto.AesEncryptByKey(codeHost.SSHKey, aesKey.PlainText)
			if err != nil {
				log.Errorf("ListCodeHost AesEncryptByKey error:%s", err)
				return nil, err
			}
		}

		if len(codeHost.PrivateAccessToken) > 0 {
			codeHost.PrivateAccessToken, err = crypto.AesEncryptByKey(codeHost.PrivateAccessToken, aesKey.PlainText)
			if err != nil {
				log.Errorf("ListCodeHost AesEncryptByKey error:%s", err)
				return nil, err
			}
		}

		result = append(result, codeHost)
	}
	return result, nil
}

func ListInternal(address, owner, source string, _ *zap.SugaredLogger) ([]*models.CodeHost, error) {
	return mongodb.NewCodehostColl().List(&mongodb.ListArgs{
		Address: address,
		Owner:   owner,
		Source:  source,
	})
}

func SystemList(encryptedKey, address, owner, source string, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	codeHosts, err := mongodb.NewCodehostColl().List(&mongodb.ListArgs{
		IntegrationLevel: setting.IntegrationLevelSystem,
		Address:          address,
		Owner:            owner,
		Source:           source,
	})
	if err != nil {
		log.Errorf("ListCodeHost error:%s", err)
		return nil, err
	}
	return EncypteCodeHost(encryptedKey, codeHosts, log)
}

func DeleteCodeHost(id int, _ *zap.SugaredLogger) error {
	return mongodb.NewCodehostColl().DeleteSystemCodeHostByID(id)
}

func UpdateCodeHost(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if host.Type == setting.SourceFromGerrit {
		host.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", host.Username, host.Password)))
	}

	var oldAlias string
	oldCodeHost, err := mongodb.NewCodehostColl().GetCodeHostByID(host.ID, false)
	if err == nil {
		oldAlias = oldCodeHost.Alias
	}
	if host.Alias != "" && host.Alias != oldAlias {
		if _, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(host.Alias); err == nil {
			return nil, fmt.Errorf("alias cannot have the same name")
		}
	}

	return mongodb.NewCodehostColl().UpdateCodeHost(host)
}

func UpdateCodeHostToken(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().UpdateCodeHostToken(host)
}

func GetCodeHost(id int, ignoreDelete bool, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().GetCodeHostByID(id, ignoreDelete)
}

type state struct {
	CodeHostID  int    `json:"code_host_id"`
	RedirectURL string `json:"redirect_url"`
}

func AuthCodeHost(redirectURI string, codeHostID int, logger *zap.SugaredLogger) (string, error) {
	codeHost, err := GetCodeHost(codeHostID, false, logger)
	if err != nil {
		logger.Errorf("GetCodeHost:%d err:%s", codeHostID, err)
		return "", err
	}
	redirectParsedURL, err := url.Parse(redirectURI)
	if err != nil {
		logger.Errorf("Parse redirectURI:%s err:%s", redirectURI, err)
		return "", err
	}
	redirectParsedURL.Path = callback
	// we need to ignore query parameters to keep grant valid
	redirectParsedURL.RawQuery = ""
	oauth, err := newOAuth(codeHost.Type, redirectParsedURL.String(), codeHost.ApplicationId, codeHost.ClientSecret, codeHost.Address)
	if err != nil {
		logger.Errorf("NewOAuth:%s err:%s", codeHost.Type, err)
		return "", err
	}
	stateStruct := state{
		CodeHostID:  codeHost.ID,
		RedirectURL: redirectURI,
	}
	bs, err := json.Marshal(stateStruct)
	if err != nil {
		logger.Errorf("Marshal err:%s", err)
		return "", err
	}
	return oauth.LoginURL(base64.URLEncoding.EncodeToString(bs)), nil
}

func HandleCallback(stateStr string, r *http.Request, logger *zap.SugaredLogger) (string, error) {
	// TODO：validate the code
	// https://www.jianshu.com/p/c7c8f51713b6
	decryptedState, err := base64.URLEncoding.DecodeString(stateStr)
	if err != nil {
		logger.Errorf("DecodeString err:%s", err)
		return "", err
	}
	var sta state
	if err := json.Unmarshal(decryptedState, &sta); err != nil {
		logger.Errorf("Unmarshal err:%s", err)
		return "", err
	}
	redirectParsedURL, err := url.Parse(sta.RedirectURL)
	if err != nil {
		logger.Errorf("ParseURL:%s err:%s", sta.RedirectURL, err)
		return "", err
	}
	codehost, err := GetCodeHost(sta.CodeHostID, false, logger)
	if err != nil {
		return handle(redirectParsedURL, err)
	}
	callbackURL := url.URL{
		Scheme: redirectParsedURL.Scheme,
		Host:   redirectParsedURL.Host,
		Path:   callback,
	}

	o, err := newOAuth(codehost.Type, callbackURL.String(), codehost.ApplicationId, codehost.ClientSecret, codehost.Address)
	if err != nil {
		logger.Errorf("newOAuth err:%s", err)
		return handle(redirectParsedURL, err)
	}
	token, err := o.HandleCallback(r, codehost)
	if err != nil {
		logger.Errorf("HandleCallback err:%s", err)
		return handle(redirectParsedURL, err)
	}
	codehost.AccessToken = token.AccessToken
	codehost.RefreshToken = token.RefreshToken
	if _, err := UpdateCodeHostToken(codehost, logger); err != nil {
		logger.Errorf("UpdateCodeHostByToken err:%s", err)
		return handle(redirectParsedURL, err)
	}
	logger.Infof("success update codehost ready status")
	return handle(redirectParsedURL, nil)
}

func newOAuth(provider, callbackURL, clientID, clientSecret, address string) (*oauth.OAuth, error) {
	switch provider {
	case setting.SourceFromGithub:
		return oauth.New(callbackURL, clientID, clientSecret, []string{"repo", "user"}, oauth2.Endpoint{
			AuthURL:  address + "/login/oauth/authorize",
			TokenURL: address + "/login/oauth/access_token",
		}), nil
	case setting.SourceFromGitlab:
		return oauth.New(callbackURL, clientID, clientSecret, []string{"api", "read_user"}, oauth2.Endpoint{
			AuthURL:  address + "/oauth/authorize",
			TokenURL: address + "/oauth/token",
		}), nil
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return oauth.New(callbackURL, clientID, clientSecret, []string{"projects", "pull_requests", "hook", "groups"}, oauth2.Endpoint{
			AuthURL:  address + "/oauth/authorize",
			TokenURL: address + "/oauth/token",
		}), nil
	}
	return nil, errors.New("illegal provider")
}

func handle(url *url.URL, err error) (string, error) {
	if err != nil {
		url.Query().Add("err", err.Error())
	} else {
		url.Query().Add("success", "true")
	}
	return url.String(), nil
}

func ValidateCodeHost(ctx *internalhandler.Context, codeHost *models.CodeHost) error {
	if codeHost.AuthType == types.SSHAuthType {
		// 解析主机地址
		user, host := getUserAndHost(codeHost.Address)
		if host == "" {
			return fmt.Errorf("无效的SSH地址: %s", codeHost.Address)
		}

		log.Debugf("user: %v, host: %v", user, host)

		// 解析端口
		port := 22
		if strings.Contains(codeHost.Address, ":") {
			parts := strings.Split(codeHost.Address, ":")
			if len(parts) > 1 {
				portStr := strings.Split(parts[1], "/")[0]
				var err error
				port, err = strconv.Atoi(portStr)
				if err != nil {
					return fmt.Errorf("无效的端口号: %s", portStr)
				}
			}
		}

		// 创建 SSH 客户端配置
		signer, err := gossh.ParsePrivateKey([]byte(codeHost.SSHKey))
		if err != nil {
			return fmt.Errorf("SSH密钥格式错误: %v", err)
		}

		config := &gossh.ClientConfig{
			User: user,
			Auth: []gossh.AuthMethod{
				gossh.PublicKeys(signer),
			},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second,
		}

		// 尝试建立 SSH 连接
		addr := fmt.Sprintf("%s:%d", host, port)
		client, err := gossh.Dial("tcp", addr, config)
		if err != nil {
			if strings.Contains(err.Error(), "ssh: handshake failed") {
				return fmt.Errorf("SSH连接失败，请确认：\n1. SSH密钥是否正确\n2. 服务器是否允许SSH连接")
			}
			return fmt.Errorf("SSH连接失败: %v", err)
		}
		defer client.Close()

		// 如果能成功建立连接并创建会话，说明 SSH 密钥是有效的
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("SSH连接验证失败: %v", err)
		}
		session.Close()

		return nil
	} else if codeHost.AuthType == types.PrivateAccessTokenAuthType {
		if codeHost.PrivateAccessToken == "" {
			return fmt.Errorf("private access token is empty")
		}

		// 尝试访问平台根目录
		req, err := http.NewRequest("GET", codeHost.Address, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败: %v", err)
		}

		// 设置认证头，同时尝试两种常见的认证方式
		req.Header.Set("Authorization", fmt.Sprintf("token %s", codeHost.PrivateAccessToken))
		req.Header.Set("PRIVATE-TOKEN", codeHost.PrivateAccessToken)

		// 发送请求
		client := &http.Client{
			Timeout: 10 * time.Second,
			// 允许自签名证书
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				return fmt.Errorf("无法连接到代码托管平台，请确认：\n1. 网络连接是否正常\n2. 平台地址是否正确")
			}
			return fmt.Errorf("请求失败: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return fmt.Errorf("Token 无效或已过期，请重新生成 Token")
			case http.StatusForbidden:
				return fmt.Errorf("Token 权限不足，请确认 Token 是否有足够的权限")
			case http.StatusNotFound:
				return fmt.Errorf("无法访问代码托管平台，请确认：\n1. 平台地址是否正确\n2. Token 是否有权限访问该平台")
			default:
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("读取响应失败: %v", err)
				}

				return fmt.Errorf("验证失败 (HTTP %d): %s", resp.StatusCode, string(body))
			}
		}

		return nil
	} else {
		return fmt.Errorf("illegal auth type: %s", codeHost.AuthType)
	}
}

func getUserAndHost(address string) (string, string) {
	user, host := "", ""

	// 处理 ssh:// 协议
	if strings.HasPrefix(address, "ssh://") {
		// 移除 ssh:// 前缀
		host = strings.TrimPrefix(address, "ssh://")
		// 如果地址中包含 @，取 @ 后面的部分
		if idx := strings.Index(host, "@"); idx != -1 {
			user = host[:idx]
			host = host[idx+1:]
		}
		// 如果地址中包含 :，取 : 前面的部分
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}
		return user, host
	}

	// 处理 git@ 格式
	parts := strings.Split(address, "@")
	if len(parts) > 1 {
		// 如果地址中包含 :，取 : 前面的部分
		user = parts[0]
		host := parts[1]
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}
		return user, host
	}

	return "", ""
}
