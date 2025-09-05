/*
Copyright 2025 The KodeRover Authors.

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

package tapd

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/util"
)

type TapdClient struct {
	Address     string
	AccessToken string
	ExpiresIn   int64
	CompanyID   string
}

func NewClient(address, clientID, clientSecret, companyID string) (*TapdClient, error) {
	tokenURL, err := url.JoinPath(address, "tokens/request_token")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	userPassword := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", clientID, clientSecret)))
	queryParams := map[string]string{
		"grant_type": "client_credentials",
	}
	result := &Response{}
	resp, err := httpclient.Post(tokenURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Basic %s", userPassword)),
		httpclient.SetBody(queryParams),
		httpclient.SetResult(result))
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get token, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return nil, fmt.Errorf("failed to get token, status code %d, err: %s", result.Status, result.Info)
	}

	token := &TokenData{}
	err = util.IToi(result.Data, &token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project info: %w", err)
	}

	return &TapdClient{
		Address:     address,
		AccessToken: token.AccessToken,
		ExpiresIn:   token.ExpiresIn,
		CompanyID:   companyID,
	}, nil
}

func (c *TapdClient) ListProjects() ([]*Workspace, error) {
	projectURL, err := url.JoinPath(c.Address, "workspaces/projects")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	ret := make([]*Workspace, 0)
	queryParams := map[string]string{
		"company_id": c.CompanyID,
		"category":   "project",
	}

	result := &Response{}
	resp, err := httpclient.Get(projectURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetQueryParams(queryParams),
		httpclient.SetResult(result))
	if err != nil {
		return nil, fmt.Errorf("failed to list project: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to list project, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return nil, fmt.Errorf("failed to list project, status code %d, err: %s", result.Status, result.Info)
	}

	projects := make([]*ProjectInfo, 0)
	err = util.IToi(result.Data, &projects)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project info: %w", err)
	}

	for _, project := range projects {
		ret = append(ret, project.Workspace)
	}

	return ret, nil
}

func (c *TapdClient) GetProject(projectID string) (*Workspace, error) {
	projectURL, err := url.JoinPath(c.Address, "workspaces/get_workspace_info")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	queryParams := map[string]string{
		"workspace_id": projectID,
	}

	result := &Response{}
	resp, err := httpclient.Get(projectURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetQueryParams(queryParams),
		httpclient.SetResult(result))
	if err != nil {
		return nil, fmt.Errorf("failed to list project: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to list project, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return nil, fmt.Errorf("failed to list project, status code %d, err: %s", result.Status, result.Info)
	}

	ret := &ProjectInfo{}
	err = util.IToi(result.Data, ret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project info: %w", err)
	}

	return ret.Workspace, nil
}

func (c *TapdClient) GetIterationCount(projectID string) (int, error) {
	iterationCountURL, err := url.JoinPath(c.Address, "iterations/count")
	if err != nil {
		return 0, fmt.Errorf("failed to join url path: %w", err)
	}

	queryParams := map[string]string{
		"workspace_id": projectID,
	}

	result := &Response{}
	resp, err := httpclient.Get(iterationCountURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetQueryParams(queryParams),
		httpclient.SetResult(result))
	if err != nil {
		return 0, fmt.Errorf("failed to get iteration count: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("failed to get iteration count, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return 0, fmt.Errorf("failed to get iteration count, status code %d, err: %s", result.Status, result.Info)
	}

	count := &GetCountResponse{}
	err = util.IToi(result.Data, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to convert sprint info: %w", err)
	}

	return count.Count, nil
}

func (c *TapdClient) ListIterations(projectID, status string) ([]*Iteration, error) {
	count, err := c.GetIterationCount(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get iteration count: %w", err)
	}

	iterationURL, err := url.JoinPath(c.Address, "iterations")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	limit := 200
	page := 1

	ret := make([]*Iteration, 0)
	for {
		queryParams := map[string]string{
			"limit":        fmt.Sprintf("%d", limit),
			"page":         fmt.Sprintf("%d", page),
			"workspace_id": projectID,
		}

		if status != "" {
			queryParams["status"] = status
		}

		result := &Response{}
		resp, err := httpclient.Get(iterationURL,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result))
		if err != nil {
			return nil, fmt.Errorf("failed to list iteration: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list iteration, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		if result.Status != 1 {
			return nil, fmt.Errorf("failed to list iteration, status code %d, err: %s", result.Status, result.Info)
		}

		iterations := make([]*IterationInfo, 0)
		err = util.IToi(result.Data, &iterations)
		if err != nil {
			return nil, fmt.Errorf("failed to convert iteration info: %w", err)
		}

		for _, iteration := range iterations {
			ret = append(ret, iteration.Iteration)
		}

		if (page-1)*limit+len(iterations) >= count {
			break
		}

		page++
	}

	return ret, nil
}

func (c *TapdClient) GetIterations(projectID string, iterationIDs []string) ([]*Iteration, error) {
	iterationURL, err := url.JoinPath(c.Address, "iterations")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	queryParams := map[string]string{
		"workspace_id": projectID,
		"limit":        fmt.Sprintf("%d", len(iterationIDs)),
		"id":           strings.Join(iterationIDs, ","),
	}

	result := &Response{}
	resp, err := httpclient.Get(iterationURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetQueryParams(queryParams),
		httpclient.SetResult(result))
	if err != nil {
		return nil, fmt.Errorf("failed to list iteration: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to list iteration, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return nil, fmt.Errorf("failed to list iteration, status code %d, err: %s", result.Status, result.Info)
	}

	iterations := make([]*IterationInfo, 0)
	err = util.IToi(result.Data, &iterations)
	if err != nil {
		return nil, fmt.Errorf("failed to convert iteration info: %w", err)
	}

	if len(iterations) != len(iterationIDs) {
		return nil, fmt.Errorf("got %d iterations, expected %d", len(iterations), len(iterationIDs))
	}

	ret := make([]*Iteration, 0)
	for _, iteration := range iterations {
		ret = append(ret, iteration.Iteration)
	}

	return ret, nil
}

func (c *TapdClient) UpdateIterationStatus(projectID, iterationID, status, user string) error {
	iterationURL, err := url.JoinPath(c.Address, "iterations")
	if err != nil {
		return fmt.Errorf("failed to join url path: %w", err)
	}

	body := &UpdateIterationStatusRequest{
		WorkspaceID: projectID,
		ID:          iterationID,
		Status:      status,
		CurrentUser: user,
	}

	result := &Response{}
	resp, err := httpclient.Post(iterationURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetBody(body),
		httpclient.SetResult(result))
	if err != nil {
		return fmt.Errorf("failed to update iteration status: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to update iteration status, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	if result.Status != 1 {
		return fmt.Errorf("failed to update iteration status, status code %d, err: %s", result.Status, result.Info)
	}

	return nil
}
