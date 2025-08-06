/*
 * Copyright 2025 The KodeRover Authors.
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

package service

import (
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/pingcode"
)

func ListPingCodeProjects(id string) ([]*pingcode.ProjectInfo, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get pingcode info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	projectList, err := client.ListProjects()
	if err != nil {
		return nil, err
	}

	return projectList, nil
}

func ListPingCodeBoards(id, projectID string) ([]*pingcode.BoardInfo, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get pingcode info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	boards, err := client.ListBoards(projectID)
	if err != nil {
		return nil, err
	}

	return boards, nil
}

func ListPingCodeSprints(id, projectID string) ([]*pingcode.SprintInfo, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get pingcode info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	sprints, err := client.ListSprints(projectID)
	if err != nil {
		return nil, err
	}

	return sprints, nil
}

func ListPingCodeWorkItemTypes(id, projectID string) ([]*pingcode.WorkItemType, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	workItemTypes, err := client.ListWorkItemTypes(projectID)
	if err != nil {
		return nil, err
	}

	return workItemTypes, nil
}

func ListPingCodeWorkItems(id, projectID string, stateIDs, sprintIDs, boardIDs, typeIDs []string, keywords string) ([]*pingcode.WorkItem, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	workItem, err := client.ListWorkItems(projectID, stateIDs, sprintIDs, boardIDs, typeIDs, keywords)
	if err != nil {
		return nil, err
	}

	return workItem, nil
}

func ListPingCodeWorkItemStates(id, projectID, fromStateID string) ([]*pingcode.WorkItemState, error) {
	pingcodeInfo, err := commonrepo.NewProjectManagementColl().GetPingCodeByID(id)
	if err != nil {
		log.Errorf("failed to get pingcode info, err: %s", err)
		return nil, err
	}
	client, err := pingcode.NewClient(pingcodeInfo.PingCodeAddress, pingcodeInfo.PingCodeClientID, pingcodeInfo.PingCodeClientSecret)
	if err != nil {
		return nil, err
	}

	states, err := client.ListWorkItemStates(projectID, fromStateID)
	if err != nil {
		return nil, err
	}

	return states, nil
}
