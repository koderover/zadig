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

type Client struct {
	*httpclient.Client
	serverAddr string
	UserName   string
	Password   string
	token      string
}

type loginResp struct {
	AccessToken string `json:"accessToken"`
	TokenTtl    int64  `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
}

type config struct {
	ID      string `json:"id"`
	DataID  string `json:"dataId"`
	Group   string `json:"group"`
	Content string `json:"content"`
	Format  string `json:"type"`
}

type configResp struct {
	PageItems []*config `json:"pageItems"`
}

type namespace struct {
	NamespaceID   string `json:"namespace"`
	NamespaceName string `json:"namespaceShowName"`
}
type namespaceResp struct {
	Data []*namespace `json:"data"`
}

const (
	// if namespace id was empty, use default namespace id
	defaultNamespaceID = "123456abcdefg"
)

func NewNacosClient(serverAddr, userName, password string) (*Client, error) {
	host, err := url.Parse(serverAddr)
	if err != nil {
		return nil, errors.Wrap(err, "parse nacos server address failed")
	}
	// add default context path
	if host.Path == "" {
		serverAddr, _ = url.JoinPath(serverAddr, "nacos")
	}
	loginURL, _ := url.JoinPath(serverAddr, "v1/auth/login")
	var result loginResp
	resp, err := req.R().AddQueryParam("username", userName).
		AddQueryParam("password", password).
		SetResult(&result).
		Post(loginURL)
	if err != nil {
		return nil, errors.Wrap(err, "login nacos failed")
	}
	if !resp.IsSuccess() {
		return nil, errors.New("login nacos failed")
	}

	c := httpclient.New(
		httpclient.SetClientHeader("accessToken", result.AccessToken),
		httpclient.SetHostURL(serverAddr),
	)

	return &Client{
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

func (c *Client) ListNamespaces() ([]*types.NacosNamespace, error) {
	url := "/v1/console/namespaces"
	res := &namespaceResp{}
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

func (c *Client) ListConfigs(namespaceID string) ([]*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v1/cs/configs"
	resp := []*types.NacosConfig{}
	pageNum := 1
	pageSize := 500
	end := false
	for !end {
		res := &configResp{}
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
		if resp, err := c.Client.Get(url, params, httpclient.SetResult(res)); err != nil {
			return nil, errors.Wrap(err, "list nacos config failed")
		} else {
			fmt.Println(string(resp.Body()))
		}
		for _, conf := range res.PageItems {
			resp = append(resp, &types.NacosConfig{
				DataID:  conf.DataID,
				Group:   conf.Group,
				Format:  getFormat(conf.Format),
				Content: conf.Content,
			})
		}
		pageNum++
		if len(res.PageItems) < pageSize {
			end = true
		}
	}
	return resp, nil
}

func (c *Client) GetConfig(dataID, group, namespaceID string) (*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	url := "/v1/cs/configs"
	res := &config{}
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
	return &types.NacosConfig{
		DataID:  res.DataID,
		Group:   res.Group,
		Format:  getFormat(res.Format),
		Content: res.Content,
	}, nil
}

func (c *Client) UpdateConfig(dataID, group, namespaceID, content, format string) error {
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
