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

package aslan

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

func (c *Client) ListWorkflows(header http.Header, qs url.Values) ([]byte, error) {
	url := "/workflow/workflow"

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) ListWorkflowsV3(header http.Header, qs url.Values) ([]byte, error) {
	url := "/workflow/v3"

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) ListAllWorkflows(header http.Header, qs url.Values) ([]byte, error) {
	url := "/workflow/v4/all"

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) ListTestWorkflows(testName string, header http.Header, qs url.Values) ([]byte, error) {
	url := fmt.Sprintf("/workflow/workflow/testName/%s", testName)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CreateWorkflowTask(header http.Header, qs url.Values, body []byte, workflowName string) ([]byte, error) {
	url := path.Join("/workflow/workflowtask", workflowName)

	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetBody(body))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CancelWorkflowTask(header http.Header, qs url.Values, id string, name string) (statusCode int, err error) {
	url := fmt.Sprintf("/workflow/workflowtask/id/%s/pipelines/%s", id, name)

	res, err := c.Delete(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return res.StatusCode(), nil
}

func (c *Client) RestartWorkflowTask(header http.Header, qs url.Values, id string, name string) (statusCode int, err error) {
	url := fmt.Sprintf("/workflow/workflowtask/id/%s/pipelines/%s/restart", id, name)

	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		log.Errorf("RestartWorkflowTask err: %s,res: %s", err, string(res.Body()))
		return http.StatusInternalServerError, err
	}

	return res.StatusCode(), nil
}

func (c *Client) ListWorkflowTask(header http.Header, qs url.Values, commitId string) ([]byte, error) {
	url := fmt.Sprintf("/workflow/v2/status/task/info?commitId=%s", commitId)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) GetDetailedWorkflowTask(header http.Header, qs url.Values, taskID, name string) ([]byte, error) {
	url := fmt.Sprintf("/workflow/workflowtask/id/%s/pipelines/%s", taskID, name)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
