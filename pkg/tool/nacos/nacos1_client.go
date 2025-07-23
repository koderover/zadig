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
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/imroc/req/v3"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
)

type NacosClient struct {
	*httpclient.Client
	serverAddr string
	UserName   string
	Password   string
	token      string
}

type nacosLoginResp struct {
	AccessToken string `json:"accessToken"`
	TokenTtl    int64  `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
}

type nacosConfig struct {
	ID      string `json:"id"`
	DataID  string `json:"dataId"`
	Group   string `json:"group"`
	Content string `json:"content"`
	Format  string `json:"type"`
}

type nacosConfigResp struct {
	PageItems []*nacosConfig `json:"pageItems"`
}

type nacosConfigHistoryResp struct {
	PageItems []*types.NacosConfigHistory `json:"pageItems"`
}

type nacosNamespace struct {
	NamespaceID   string `json:"namespace"`
	NamespaceName string `json:"namespaceShowName"`
}
type nacosNamespaceResp struct {
	Data []*nacosNamespace `json:"data"`
}

const (
	// if namespace id was empty, use default namespace id
	defaultNamespaceID = "123456abcdefg"
)

func NewNacos1Client(serverAddr, userName, password string) (*NacosClient, error) {
	host, err := url.Parse(serverAddr)
	if err != nil {
		return nil, errors.Wrap(err, "parse nacos server address failed")
	}
	// add default context path
	if host.Path == "" {
		serverAddr, _ = url.JoinPath(serverAddr, "nacos")
	}
	loginURL, _ := url.JoinPath(serverAddr, "v1/auth/login")
	var result nacosLoginResp
	resp, err := req.R().AddQueryParam("username", userName).
		AddQueryParam("password", password).
		SetResult(&result).
		Post(loginURL)
	if err != nil {
		return nil, errors.Wrap(err, "login nacos failed")
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("login nacos failed: %s", resp.String())
	}

	c := httpclient.New(
		httpclient.SetClientHeader("accessToken", result.AccessToken),
		httpclient.SetHostURL(serverAddr),
	)

	return &NacosClient{
		Client:     c,
		serverAddr: serverAddr,
		token:      result.AccessToken,
		UserName:   userName,
		Password:   password,
	}, nil
}

func setNamespaceID(namespaceID string) string {
	if namespaceID == "" {
		return defaultNamespaceID
	}
	return namespaceID
}

func getNamespaceID(namespaceID string) string {
	if namespaceID == defaultNamespaceID {
		return ""
	}
	return namespaceID
}

func (c *NacosClient) ListNamespaces() ([]*types.NacosNamespace, error) {
	url := "/v1/console/namespaces"
	res := &nacosNamespaceResp{}
	if _, err := c.Client.Get(url, httpclient.SetResult(res)); err != nil {
		return nil, errors.Wrap(err, "list nacos namespace failed")
	}
	resp := []*types.NacosNamespace{}
	for _, namespace := range res.Data {
		resp = append(resp, &types.NacosNamespace{
			NamespaceID:    setNamespaceID(namespace.NamespaceID),
			NamespacedName: namespace.NamespaceName,
		})
	}
	return resp, nil
}

func (c *NacosClient) ListConfigs(namespaceID string) ([]*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v1/cs/configs"
	resp := []*types.NacosConfig{}
	pageNum := 1
	pageSize := 500
	end := false
	for !end {
		res := &nacosConfigResp{}
		numString := strconv.Itoa(pageNum)
		sizeString := strconv.Itoa(pageSize)
		params := httpclient.SetQueryParams(map[string]string{
			"dataId":      "",
			"group":       "",
			"search":      "accurate",
			"pageNo":      numString,
			"pageSize":    sizeString,
			"tenant":      namespaceID,
			"accessToken": c.token,
		})
		if _, err := c.Client.Get(url, params, httpclient.SetResult(res)); err != nil {
			return nil, errors.Wrap(err, "list nacos config failed")
		}
		for _, conf := range res.PageItems {
			nacosID := types.NacosDataID{
				DataID: conf.DataID,
				Group:  conf.Group,
			}
			resp = append(resp, &types.NacosConfig{
				NacosDataID: nacosID,
			})
		}
		pageNum++
		if len(res.PageItems) < pageSize {
			end = true
		}
	}
	return resp, nil
}

func (c *NacosClient) GetConfig(dataID, group, namespaceID string) (*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v1/cs/configs"
	res := &nacosConfig{}
	params := httpclient.SetQueryParams(map[string]string{
		"dataId":      dataID,
		"group":       group,
		"tenant":      namespaceID,
		"show":        "all",
		"accessToken": c.token,
	})
	if _, err := c.Client.Get(url, params, httpclient.SetResult(res)); err != nil {
		return nil, errors.Wrap(err, "get nacos config failed")
	}
	nacosID := types.NacosDataID{
		DataID: res.DataID,
		Group:  res.Group,
	}
	return &types.NacosConfig{
		NacosDataID: nacosID,
		Format:      getFormat(res.Format),
		Content:     res.Content,
	}, nil
}

func (c *NacosClient) GetConfigHistory(dataID, group, namespaceID string) ([]*types.NacosConfigHistory, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v1/cs/history"

	params := httpclient.SetQueryParams(map[string]string{
		"dataId":      dataID,
		"group":       group,
		"tenant":      namespaceID,
		"search":      "accurate",
		"accessToken": c.token,
	})

	res := &nacosConfigHistoryResp{}
	if _, err := c.Client.Get(url, params, httpclient.SetResult(res)); err != nil {
		return nil, errors.Wrap(err, "list nacos config history failed")
	}

	return res.PageItems, nil
}

func (c *NacosClient) UpdateConfig(dataID, group, namespaceID, content, format string) error {
	namespaceID = getNamespaceID(namespaceID)
	path := "/v1/cs/configs"
	formValues := map[string]string{
		"dataId":      dataID,
		"group":       group,
		"tenant":      namespaceID,
		"content":     content,
		"type":        setFormat(format),
		"accessToken": c.token,
	}
	if _, err := c.Client.Post(path, httpclient.SetFormData(formValues)); err != nil {
		return errors.Wrap(err, "update nacos config failed")
	}
	return nil
}

func (c *NacosClient) Validate() error {
	return nil
}

func getFormat(format string) string {
	switch strings.ToLower(format) {
	case "yaml":
		return "YAML"
	case "json":
		return "JSON"
	case "properties":
		return "Properties"
	case "xml":
		return "XML"
	case "html":
		return "HTML"
	default:
		return "TEXT"
	}
}

func setFormat(format string) string {
	switch strings.ToLower(format) {
	case "yaml":
		return "yaml"
	case "json":
		return "json"
	case "properties":
		return "properties"
	case "xml":
		return "xml"
	case "html":
		return "html"
	default:
		return "text"
	}
}
