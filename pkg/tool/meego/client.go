/*
 * Copyright 2023 The KodeRover Authors.
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

package meego

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Client struct {
	Host         string
	PluginID     string
	PluginSecret string
	PluginToken  string
	UserKey      string
}

type GetPluginTokenRequest struct {
	PluginID     string `json:"plugin_id"`
	PluginSecret string `json:"plugin_secret"`
	Type         int    `json:"type"`
}

type GetPluginTokenResult struct {
	Data  *GetPluginData `json:"data"`
	Error *GeneralError  `json:"error"`
}

type GetPluginData struct {
	Token      string `json:"token"`
	ExpireTime int64  `json:"expire_time"`
}

type GeneralError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

// Since we only allow one meego integration at a time, we store the pluginToken in the memory
var TokenExpirationMap sync.Map

func NewClient(host, pluginID, pluginSecret, userKey string) (*Client, error) {
	meegoHost := host
	if meegoHost == "" {
		meegoHost = DefaultMeegoHost
	}
	expirationTimeCombination, ok := TokenExpirationMap.Load(pluginID)
	// if we can't find an expiration time, it means either this is the first client, or the
	// service has restarted. Either way, we do a token exchange
	if !ok {
		token, expirationDuration, err := GetPluginToken(host, pluginID, pluginSecret)
		if err != nil {
			return nil, err
		}

		// we have 5 minute as a buffer in case something bad happens
		expirationTime := time.Now().Unix() + expirationDuration - 300
		mapValue := fmt.Sprintf("%s_%d", token, expirationTime)
		TokenExpirationMap.Store(pluginID, mapValue)

		return &Client{
			Host:         meegoHost,
			PluginID:     pluginID,
			PluginSecret: pluginSecret,
			PluginToken:  token,
			UserKey:      userKey,
		}, nil
	}

	// otherwise we need to check if the token is expired
	kv := strings.Split(expirationTimeCombination.(string), "_")
	if len(kv) != 2 {
		errorMsg := fmt.Sprintf("%s: %s", "meego token in memory corrupted, token: ", expirationTimeCombination.(string))
		return nil, errors.New(errorMsg)
	}
	expirationTimeStr := kv[1]
	expirationTime, err := strconv.ParseInt(expirationTimeStr, 10, 64)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", "failed to get the expiration time of meego token, expiration time is", expirationTimeStr)
		return nil, errors.New(errorMsg)
	}
	if time.Now().Unix() <= expirationTime {
		return &Client{
			Host:         meegoHost,
			PluginID:     pluginID,
			PluginSecret: pluginSecret,
			PluginToken:  kv[0],
			UserKey:      userKey,
		}, nil
	} else {
		// if the token already expires, we refresh this
		token, expirationDuration, err := GetPluginToken(host, pluginID, pluginSecret)
		if err != nil {
			return nil, err
		}

		// we have 5 minute as a buffer in case something bad happens
		expirationTime := time.Now().Unix() + expirationDuration - 300
		mapValue := fmt.Sprintf("%s_%d", token, expirationTime)
		TokenExpirationMap.Store(pluginID, mapValue)

		return &Client{
			Host:         meegoHost,
			PluginID:     pluginID,
			PluginSecret: pluginSecret,
			PluginToken:  token,
			UserKey:      userKey,
		}, nil
	}
}

func GetPluginToken(host, pluginID, pluginSecret string) (token string, expirationTime int64, err error) {
	meegoHost := host
	if meegoHost == "" {
		meegoHost = DefaultMeegoHost
	}

	tokenExchangeAPI := fmt.Sprintf("%s/open_api/authen/plugin_token", meegoHost)

	body := &GetPluginTokenRequest{
		PluginID:     pluginID,
		PluginSecret: pluginSecret,
		Type:         PluginTokenType,
	}

	result := new(GetPluginTokenResult)
	_, err = httpclient.Post(tokenExchangeAPI, httpclient.SetBody(body), httpclient.SetResult(result))
	if err != nil {
		log.Errorf("error occured when getting temporary plugin, error: ", err)
		return "", 0, err
	}

	if result.Error.Code != 0 {
		// if the error code is not 0, then return nil, and an error
		return "", 0, errors.New(result.Error.Message)
	}

	return result.Data.Token, result.Data.ExpireTime, nil
}
