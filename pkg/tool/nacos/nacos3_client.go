/*
Copyright 2023 The KodeRover Authors.

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

package nacos

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/imroc/req/v3"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Nacos3Client struct {
	*httpclient.Client
	serverAddr string
	UserName   string
	Password   string
	token      string
}

type nacos3Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type nacos3Namespace struct {
	Namespace         string `json:"namespace"`
	NamespaceShowName string `json:"namespaceShowName"`
	NamespaceDesc     string `json:"namespaceDesc"`
	ConfigCount       int    `json:"configCount"`
	Quota             int    `json:"quota"`
	Type              int    `json:"type"`
}

type nacos3ConfigItem struct {
	DataID    string `json:"dataId"`
	GroupName string `json:"groupName"`
}

type nacos3Config struct {
	ID                string `json:"id"`
	DataID            string `json:"dataId"`
	GroupName         string `json:"groupName"`
	NamespaceID       string `json:"namespaceId"`
	Content           string `json:"content"`
	Desc              string `json:"desc"`
	MD5               string `json:"md5"`
	ConfigTags        string `json:"configTags"`
	EncryptedDataKey  string `json:"encryptedDataKey"`
	EncryptedDataSalt string `json:"encryptedDataSalt"`
	AppName           string `json:"appName"`
	Type              string `json:"type"`
	CreateTime        int64  `json:"createTime"`
	ModifyTime        int64  `json:"modifyTime"`
	CreateUser        string `json:"createUser"`
	CreateIp          string `json:"createIp"`
}

type nacos3ConfigHistoryResp struct {
	PageItems []*nacos3ConfigHistory `json:"pageItems"`
}

type nacos3ConfigHistory struct {
	ID           string `json:"id"`
	DataID       string `json:"dataId"`
	GroupName    string `json:"groupName"`
	NamespaceID  string `json:"namespaceId"`
	Content      string `json:"content"`
	AppName      string `json:"appName"`
	OpType       string `json:"opType"`
	PublishType  string `json:"publishType"`
	SrcIP        string `json:"srcIp"`
	SrcUser      string `json:"srcUser"`
	CreatedTime  int64  `json:"createdTime"`
	ModifiedTime int64  `json:"modifiedTime"`
}

func NewNacos3Client(serverAddr, userName, password string) (*Nacos3Client, error) {
	loginURL, _ := url.JoinPath(serverAddr, "/v3/auth/user/login")

	nacosResp := &nacos3Resp{}
	resp, err := req.R().AddQueryParam("username", userName).
		AddQueryParam("password", password).
		SetResult(nacosResp).
		Post(loginURL)
	if err != nil {
		return nil, errors.Wrap(err, "login nacos failed")
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("login nacos failed: %s", resp.String())
	}

	if err := nacosResp.handleError(); err != nil {
		return nil, errors.Wrap(err, "login nacos failed")
	}

	result := nacosLoginResp{}
	if err := resp.UnmarshalJson(&result); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos login response failed")
	}

	c := httpclient.New(
		httpclient.SetClientHeader("accessToken", result.AccessToken),
		httpclient.SetHostURL(serverAddr),
	)

	return &Nacos3Client{
		Client:     c,
		serverAddr: serverAddr,
		token:      result.AccessToken,
		UserName:   userName,
		Password:   password,
	}, nil
}

func (c *Nacos3Client) ListNamespaces() ([]*types.NacosNamespace, error) {
	url := "/v3/console/core/namespace/list"

	nacosResp := &nacos3Resp{}
	if _, err := c.Client.Get(url, httpclient.SetResult(nacosResp)); err != nil {
		return nil, errors.Wrap(err, "list nacos namespace failed")
	}

	if err := nacosResp.handleError(); err != nil {
		return nil, errors.Wrap(err, "list nacos namespace failed")
	}

	res := []*nacos3Namespace{}
	if err := IToi(nacosResp.Data, &res); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos namespace response failed")
	}

	resp := []*types.NacosNamespace{}
	for _, namespace := range res {
		resp = append(resp, &types.NacosNamespace{
			NamespaceID:    setNamespaceID(namespace.Namespace),
			NamespacedName: namespace.NamespaceShowName,
		})
	}
	return resp, nil
}

func (c *Nacos3Client) ListConfigs(namespaceID string) ([]*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v3/console/cs/history/configs"

	nacosResp := &nacos3Resp{}
	params := httpclient.SetQueryParams(map[string]string{
		"namespaceId": namespaceID,
		"accessToken": c.token,
	})
	if _, err := c.Client.Get(url, params, httpclient.SetResult(nacosResp)); err != nil {
		return nil, errors.Wrap(err, "list nacos config failed")
	}

	if err := nacosResp.handleError(); err != nil {
		return nil, errors.Wrap(err, "list nacos config failed")
	}

	res := []*nacos3ConfigItem{}
	if err := IToi(nacosResp.Data, &res); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos config response failed")
	}

	configs := []*types.NacosConfig{}
	for _, config := range res {
		configs = append(configs, &types.NacosConfig{
			NacosDataID: types.NacosDataID{
				DataID: config.DataID,
				Group:  config.GroupName,
			},
			NamespaceID: namespaceID,
		})
	}

	return configs, nil
}

func (c *Nacos3Client) GetConfig(dataID, group, namespaceID string) (*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v3/console/cs/config"
	params := httpclient.SetQueryParams(map[string]string{
		"dataId":      dataID,
		"groupName":   group,
		"namespaceId": namespaceID,
		"accessToken": c.token,
	})

	nacosResp := &nacos3Resp{}
	if _, err := c.Client.Get(url, params, httpclient.SetResult(nacosResp)); err != nil {
		return nil, errors.Wrap(err, "get nacos config failed")
	}

	if err := nacosResp.handleError(); err != nil {
		return nil, errors.Wrap(err, "get nacos config failed")
	}

	res := &nacos3Config{}
	if err := IToi(nacosResp.Data, res); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos config response failed")
	}

	nacosID := types.NacosDataID{
		DataID: res.DataID,
		Group:  res.GroupName,
	}
	return &types.NacosConfig{
		NacosDataID: nacosID,
		Format:      getFormat(res.Type),
		Content:     res.Content,
	}, nil
}

func (c *Nacos3Client) GetConfigHistory(dataID, group, namespaceID string) ([]*types.NacosConfigHistory, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v3/console/cs/history/list"

	params := httpclient.SetQueryParams(map[string]string{
		"dataId":      dataID,
		"groupName":   group,
		"namespaceId": namespaceID,
		"accessToken": c.token,
	})

	nacosResp := &nacos3Resp{}
	if _, err := c.Client.Get(url, params, httpclient.SetResult(nacosResp)); err != nil {
		return nil, errors.Wrap(err, "list nacos config history failed")
	}

	if err := nacosResp.handleError(); err != nil {
		return nil, errors.Wrap(err, "list nacos config history failed")
	}

	res := &nacos3ConfigHistoryResp{}
	if err := IToi(nacosResp.Data, res); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos config history response failed")
	}

	histories := []*types.NacosConfigHistory{}
	historyMap := make(map[string]*types.NacosConfigHistory)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	for _, history := range res.PageItems {
		history := history // 闭包变量
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			url := "/v3/console/cs/history"
			params := httpclient.SetQueryParams(map[string]string{
				"dataId":      history.DataID,
				"groupName":   history.GroupName,
				"namespaceId": namespaceID,
				"nid":         history.ID,
				"accessToken": c.token,
			})

			nacosResp := &nacos3Resp{}
			if _, err := c.Client.Get(url, params, httpclient.SetResult(nacosResp)); err != nil {
				return errors.Wrap(err, "get nacos config history failed")
			}

			if err := nacosResp.handleError(); err != nil {
				return errors.Wrap(err, "get nacos config history failed")
			}

			res := &nacos3ConfigHistory{}
			if err := IToi(nacosResp.Data, res); err != nil {
				return errors.Wrap(err, "unmarshal nacos config history response failed")
			}

			mu.Lock()
			historyMap[history.ID] = &types.NacosConfigHistory{
				ID:      history.ID,
				DataID:  history.DataID,
				Group:   history.GroupName,
				AppName: history.AppName,
				Tenant:  history.NamespaceID,
				Content: res.Content,
				OpType:  history.OpType,
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, history := range historyMap {
		histories = append(histories, history)
	}

	return histories, nil
}

func (c *Nacos3Client) UpdateConfig(dataID, group, namespaceID, content, format string) error {
	namespaceID = getNamespaceID(namespaceID)
	path := "/v3/console/cs/config"
	formValues := map[string]string{
		"namespaceId": namespaceID,
		"groupName":   group,
		"dataId":      dataID,
		"content":     content,
		"type":        setFormat(format),
		"accessToken": c.token,
	}

	nacosResp := &nacos3Resp{}
	if _, err := c.Client.Post(path, httpclient.SetFormData(formValues), httpclient.SetResult(nacosResp)); err != nil {
		return errors.Wrap(err, "update nacos config failed")
	}

	if err := nacosResp.handleError(); err != nil {
		return errors.Wrap(err, "update nacos config failed")
	}

	return nil
}

func (c *Nacos3Client) Validate() error {
	return nil
}

func (r *nacos3Resp) handleError() error {
	if r.Code != 0 {
		return errors.New(r.Message)
	}

	return nil
}
