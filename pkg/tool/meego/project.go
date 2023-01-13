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

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type GetNamespaceRequest struct {
	UserKey string `json:"user_key"`
}

type GetNamespaceResult struct {
	Data         []string    `json:"data"`
	ErrorMessage string      `json:"err_msg"`
	ErrorCode    int         `json:"err_code"`
	Error        interface{} `json:"error"`
}

type GetNamespaceDetailRequest struct {
	ProjectKeys []string `json:"project_keys"`
	UserKey     string   `json:"user_key"`
}

type GetNamespaceDetailResult struct {
	Data         map[string]*NamespaceDetail `json:"data"`
	ErrorMessage string                      `json:"err_msg"`
	ErrorCode    int                         `json:"err_code"`
	Error        interface{}                 `json:"error"`
}

type NamespaceDetail struct {
	Admins     []string `json:"administrators"`
	Name       string   `json:"name"`
	ProjectKey string   `json:"project_key"`
	SimpleName string   `json:"simple_name"`
}

func (c *Client) GetProjectList() ([]*NamespaceDetail, error) {
	getProjectListAPI := fmt.Sprintf("%s/open_api/projects", c.Host)

	request := &GetNamespaceRequest{
		UserKey: c.UserKey,
	}

	result := new(GetNamespaceResult)

	_, err := httpclient.Post(getProjectListAPI,
		httpclient.SetBody(request),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("failed to get meego project list, error: %s", err)
		return nil, err
	}

	if result.ErrorCode != 0 {
		errorMsg := fmt.Sprintf("error response when getting meego project list, error code: %d, error message: %s, err: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}

	getProjectDetailAPI := fmt.Sprintf("%s/open_api/projects/detail", c.Host)

	detailRequest := &GetNamespaceDetailRequest{
		UserKey:     c.UserKey,
		ProjectKeys: result.Data,
	}

	detailResult := new(GetNamespaceDetailResult)

	_, err = httpclient.Post(getProjectDetailAPI,
		httpclient.SetBody(detailRequest),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(detailResult),
	)

	if err != nil {
		log.Errorf("failed to get meego project detail list, error: %s", err)
		return nil, err
	}

	if result.ErrorCode != 0 {
		errorMsg := fmt.Sprintf("error response when getting meego project detail list, error code: %d, error message: %s, err: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}

	resp := make([]*NamespaceDetail, 0)
	for _, detailInfo := range detailResult.Data {
		resp = append(resp, detailInfo)
	}
	return resp, nil
}

type GetWorkItemTypeResp struct {
	Data         []*WorkItemType `json:"data"`
	ErrorMessage string          `json:"err_msg"`
	ErrorCode    int             `json:"err_code"`
	Error        interface{}     `json:"error"`
}

type WorkItemType struct {
	TypeKey string `json:"type_key"`
	Name    string `json:"name"`
}

func (c *Client) GetWorkItemTypesList(projectKey string) ([]*WorkItemType, error) {
	api := fmt.Sprintf("%s/open_api/%s/work_item/all-types", c.Host, projectKey)

	result := new(GetWorkItemTypeResp)

	_, err := httpclient.Get(api,
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("error occured when getting meego workload item types, error: %s", err)
		return nil, err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when getting meego item types, error code: %d, error message: %s, err: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	return result.Data, nil
}
