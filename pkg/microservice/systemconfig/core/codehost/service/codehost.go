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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/internal/oauth"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

const callback = "/api/directory/codehosts/callback"

func CreateCodeHost(codehost *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if codehost.Type == setting.SourceFromCodeHub || codehost.Type == setting.SourceFromOther {
		codehost.IsReady = "2"
	}
	if codehost.Type == setting.SourceFromGerrit {
		codehost.IsReady = "2"
		codehost.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", codehost.Username, codehost.Password)))
	}

	if codehost.Alias != "" {
		if _, err := mongodb.NewCodehostColl().GetCodeHostByAlias(codehost.Alias); err == nil {
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
	return mongodb.NewCodehostColl().AddCodeHost(codehost)
}

func encypteCodeHost(encryptedKey string, codeHosts []*models.CodeHost, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	aesKey, err := aslan.New(config.AslanServiceAddress()).GetTextFromEncryptedKey(encryptedKey)
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

func List(encryptedKey, address, owner, source string, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	codeHosts, err := mongodb.NewCodehostColl().List(&mongodb.ListArgs{
		Address: address,
		Owner:   owner,
		Source:  source,
	})
	if err != nil {
		log.Errorf("ListCodeHost error:%s", err)
		return nil, err
	}
	return encypteCodeHost(encryptedKey, codeHosts, log)
}

func DeleteCodeHost(id int, _ *zap.SugaredLogger) error {
	return mongodb.NewCodehostColl().DeleteCodeHostByID(id)
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
		if _, err := mongodb.NewCodehostColl().GetCodeHostByAlias(host.Alias); err == nil {
			return nil, fmt.Errorf("alias cannot have the same name")
		}
	}

	return mongodb.NewCodehostColl().UpdateCodeHost(host)
}

func UpdateCodeHostByToken(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().UpdateCodeHostByToken(host)
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
	if _, err := UpdateCodeHostByToken(codehost, logger); err != nil {
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
