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

package pingcode

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/util"
	"k8s.io/apimachinery/pkg/util/sets"
)

type PingCodeClient struct {
	Address     string
	AccessToken string
	ExpiresIn   int64
}

func NewClient(address, clientID, clientSecret string) (*PingCodeClient, error) {
	tokenURL, err := url.JoinPath(address, "v1/auth/token")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	queryParams := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     clientID,
		"client_secret": clientSecret,
	}
	result := &GetTokenResponse{}
	resp, err := httpclient.Get(tokenURL, httpclient.SetQueryParams(queryParams), httpclient.SetResult(result))
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get token, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	return &PingCodeClient{
		Address:     address,
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
	}, nil
}

func (c *PingCodeClient) ListProjects() ([]*ProjectInfo, error) {
	projectURL, err := url.JoinPath(c.Address, "v1/project/projects")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := make([]*ProjectInfo, 0)
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
		}

		result := &PaginationResponse{}
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

		projects := make([]*ProjectInfo, 0)
		err = util.IToi(result.Values, &projects)
		if err != nil {
			return nil, fmt.Errorf("failed to convert project info: %w", err)
		}

		ret = append(ret, projects...)

		if result.PageIndex*result.PageSize+len(projects) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) ListBoards(projectID string) ([]*BoardInfo, error) {
	boardURL, err := url.JoinPath(c.Address, "v1/project/projects", projectID, "boards")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := make([]*BoardInfo, 0)
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
		}

		result := &PaginationResponse{}
		resp, err := httpclient.Get(boardURL,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result))
		if err != nil {
			return nil, fmt.Errorf("failed to list board: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list board, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		boards := make([]*BoardInfo, 0)
		err = util.IToi(result.Values, &boards)
		if err != nil {
			return nil, fmt.Errorf("failed to convert board info: %w", err)
		}

		ret = append(ret, boards...)

		if result.PageIndex*result.PageSize+len(boards) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) ListSprints(projectID string) ([]*SprintInfo, error) {
	sprintURL, err := url.JoinPath(c.Address, "v1/project/projects", projectID, "sprints")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := make([]*SprintInfo, 0)
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
		}

		result := &PaginationResponse{}
		resp, err := httpclient.Get(sprintURL,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result))
		if err != nil {
			return nil, fmt.Errorf("failed to list sprint: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list sprint, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		sprints := make([]*SprintInfo, 0)
		err = util.IToi(result.Values, &sprints)
		if err != nil {
			return nil, fmt.Errorf("failed to convert sprint info: %w", err)
		}

		ret = append(ret, sprints...)

		if result.PageIndex*result.PageSize+len(sprints) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) ListWorkItemTypes(projectID string) ([]*WorkItemType, error) {
	workItemTypeURL, err := url.JoinPath(c.Address, "v1/project/work_item/types")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := make([]*WorkItemType, 0)
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
			"project_id": projectID,
		}

		result := &PaginationResponse{}
		resp, err := httpclient.Get(workItemTypeURL,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list work item type: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list work item type, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		workItemTypes := make([]*WorkItemType, 0)
		err = util.IToi(result.Values, &workItemTypes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert work item type: %w", err)
		}

		ret = append(ret, workItemTypes...)

		if result.PageIndex*result.PageSize+len(workItemTypes) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) ListWorkItems(projectID string, stateIDs, sprintIDs, boardIDs, typeIDs []string, keywords string) ([]*WorkItem, error) {
	workItemTypeURL, err := url.JoinPath(c.Address, "v1/project/work_items")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := make([]*WorkItem, 0)
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
		}
		if projectID != "" {
			queryParams["project_ids"] = projectID
		}
		if len(stateIDs) > 20 || len(sprintIDs) > 20 || len(boardIDs) > 20 || len(typeIDs) > 20 {
			return nil, fmt.Errorf("stateIDs or sprintIDs or boardIDs or typeIDs count large than 20")
		}
		if len(stateIDs) != 0 {
			queryParams["state_ids"] = strings.Join(stateIDs, ",")
		}
		if len(sprintIDs) != 0 {
			queryParams["sprint_ids"] = strings.Join(sprintIDs, ",")
		}
		if len(boardIDs) != 0 {
			queryParams["boards_ids"] = strings.Join(boardIDs, ",")
		}
		if len(typeIDs) != 0 {
			queryParams["type_ids"] = strings.Join(typeIDs, ",")
		}
		if keywords != "" {
			queryParams["keywords"] = keywords
		}

		result := &PaginationResponse{}
		resp, err := httpclient.Get(workItemTypeURL,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list work item: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list work item, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		workItems := make([]*WorkItem, 0)
		err = util.IToi(result.Values, &workItems)
		if err != nil {
			return nil, fmt.Errorf("failed to convert work item: %w", err)
		}

		ret = append(ret, workItems...)

		if result.PageIndex*result.PageSize+len(workItems) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) ListWorkItemStates(projectID, fromStateID string) ([]*WorkItemState, error) {
	planUrl, err := url.JoinPath(c.Address, "v1/project/work_item_state_plans")
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %w", err)
	}

	pageSize := 100
	pageIndex := 0

	ret := []*WorkItemState{}
	stateIDSet := sets.NewString()
	for {
		queryParams := map[string]string{
			"page_size":  fmt.Sprintf("%d", pageSize),
			"page_index": fmt.Sprintf("%d", pageIndex),
			"project_id": projectID,
		}

		result := &PaginationResponse{}
		resp, err := httpclient.Get(planUrl,
			httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
			httpclient.SetQueryParams(queryParams),
			httpclient.SetResult(result),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list status plan: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to list status plan, status code %d, err: %s", resp.StatusCode(), resp.String())
		}

		plans := make([]*WorkItemStatePlan, 0)
		err = util.IToi(result.Values, &plans)
		if err != nil {
			return nil, fmt.Errorf("failed to convert status plan: %w", err)
		}

		for _, plan := range plans {
			stateUrl, err := url.JoinPath(c.Address, "v1/project/work_item_state_plans/", plan.ID, "/work_item_state_flows")
			if err != nil {
				return nil, fmt.Errorf("failed to join url path: %w", err)
			}

			subPageSize := 100
			subPageIndex := 0

			for {
				queryParams := map[string]string{
					"page_size":  fmt.Sprintf("%d", subPageSize),
					"page_index": fmt.Sprintf("%d", subPageIndex),
				}

				if fromStateID != "" {
					queryParams["from_state_id"] = fromStateID
				}

				result := &PaginationResponse{}
				resp, err := httpclient.Get(stateUrl,
					httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
					httpclient.SetQueryParams(queryParams),
					httpclient.SetResult(result),
				)
				if err != nil {
					return nil, fmt.Errorf("failed to list status: %w", err)
				}

				if resp.StatusCode() != http.StatusOK {
					return nil, fmt.Errorf("failed to list status, status code %d, err: %s", resp.StatusCode(), resp.String())
				}

				flows := make([]*ListWorkItemStateFlows, 0)
				err = util.IToi(result.Values, &flows)
				if err != nil {
					return nil, fmt.Errorf("failed to convert status plan: %w", err)
				}

				for _, flow := range flows {
					if flow.ToState == nil {
						continue
					}
					if stateIDSet.Has(flow.ToState.ID) {
						continue
					}

					if fromStateID != "" {
						if flow.FromState != nil && flow.FromState.ID == fromStateID {
							ret = append(ret, flow.ToState)
							stateIDSet.Insert(flow.ToState.ID)
						}
					} else {
						ret = append(ret, flow.ToState)
						stateIDSet.Insert(flow.ToState.ID)
					}
				}

				if result.PageIndex*result.PageSize+len(flows) >= result.Total {
					break
				}

				subPageIndex++
			}
		}

		if result.PageIndex*result.PageSize+len(plans) >= result.Total {
			break
		}

		pageIndex++
	}

	return ret, nil
}

func (c *PingCodeClient) UpdateWorkItemState(workItemID, stateID string) error {
	workItemURL, err := url.JoinPath(c.Address, "v1/project/work_items/", workItemID)
	if err != nil {
		return fmt.Errorf("failed to join url path: %w", err)
	}

	body := &UpdateWorkItemStateRequest{
		StateID: stateID,
	}
	resp, err := httpclient.Patch(workItemURL,
		httpclient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken)),
		httpclient.SetBody(body),
	)
	if err != nil {
		return fmt.Errorf("更新工作项状态失败: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("更新工作项状态失败, status code %d, err: %s", resp.StatusCode(), resp.String())
	}

	return nil
}
