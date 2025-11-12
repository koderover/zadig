/*
 * Copyright 2022 The KodeRover Authors.
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
	"fmt"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
)

func GetMeegoProjects(id string) (*MeegoProjectResp, error) {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		return nil, err
	}

	projectList, err := client.GetProjectList()
	if err != nil {
		return nil, err
	}

	meegoProjectList := make([]*MeegoProject, 0)
	for _, project := range projectList {
		meegoProjectList = append(meegoProjectList, &MeegoProject{
			Name: project.Name,
			Key:  project.ProjectKey,
		})
	}
	return &MeegoProjectResp{Projects: meegoProjectList}, nil
}

func GetWorkItemTypeList(id, projectID string) (*MeegoWorkItemTypeResp, error) {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		return nil, err
	}

	workItemTypeList, err := client.GetWorkItemTypesList(projectID)
	if err != nil {
		return nil, err
	}

	meegoWorkItemTypeList := make([]*MeegoWorkItemType, 0)
	for _, workItemType := range workItemTypeList {
		meegoWorkItemTypeList = append(meegoWorkItemTypeList, &MeegoWorkItemType{
			TypeKey: workItemType.TypeKey,
			Name:    workItemType.Name,
		})
	}
	return &MeegoWorkItemTypeResp{WorkItemTypes: meegoWorkItemTypeList}, nil
}

func ListMeegoWorkItems(id, projectID, typeKey, nameQuery string, pageNum, pageSize int) (*MeegoWorkItemResp, error) {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		return nil, err
	}

	workItemList, err := client.GetWorkItemList(projectID, typeKey, nameQuery, pageNum, pageSize)
	if err != nil {
		return nil, err
	}

	meegoWorkItemList := make([]*MeegoWorkItem, 0)
	for _, workItem := range workItemList {
		meegoWorkItemList = append(meegoWorkItemList, &MeegoWorkItem{
			ID:           workItem.ID,
			Name:         workItem.Name,
			Pattern:      workItem.Pattern,
			CurrentState: workItem.WorkItemStatus.StateKey,
		})
	}

	return &MeegoWorkItemResp{WorkItems: meegoWorkItemList}, nil
}

type ListMeegoWorkItemNodesResp struct {
	Nodes []*meego.WorkflowNode `json:"nodes"`
}

func ListMeegoWorkItemNodes(id, projectID, typeKey string, workitemID int) (*ListMeegoWorkItemNodesResp, error) {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		err = fmt.Errorf("failed to get meego info, err: %s", err)
		log.Error(err)
		return nil, err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		err = fmt.Errorf("failed to new meego client, err: %s", err)
		log.Error(err)
		return nil, err
	}

	_, nodes, _, err := client.GetWorkFlowInfo(projectID, typeKey, meego.WorkItemPatternNode, workitemID)
	if err != nil {
		err = fmt.Errorf("failed to get workflow info, err: %s", err)
		log.Error(err)
		return nil, err
	}

	return &ListMeegoWorkItemNodesResp{Nodes: nodes}, nil
}

func ConfirmWorkItemNode(id, projectID, typeKey, workItemID, nodeID string) error {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		return err
	}

	err = client.NodeOperate(projectID, typeKey, workItemID, nodeID)
	if err != nil {
		return err
	}

	return nil
}

func ListAvailableWorkItemTransitions(id, projectID, typeKey string, pattern meego.WorkItemPattern, workItemID int) (*MeegoTransitionResp, error) {
	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(id)
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		return nil, err
	}

	workItem, err := client.GetWorkItem(projectID, typeKey, workItemID)
	if err != nil {
		return nil, err
	}

	transitions, _, stateInfoList, err := client.GetWorkFlowInfo(projectID, typeKey, pattern, workItemID)
	if err != nil {
		return nil, err
	}

	targetStateMap := make(map[string]string)
	for _, stateInfo := range stateInfoList {
		targetStateMap[stateInfo.ID] = stateInfo.Name
	}

	availableTransition := make([]*MeegoWorkItemStatusTransition, 0)
	for _, transition := range transitions {
		if workItem.WorkItemStatus.StateKey == transition.SourceStateKey {
			availableTransition = append(availableTransition, &MeegoWorkItemStatusTransition{
				SourceStateKey:  transition.SourceStateKey,
				SourceStateName: targetStateMap[transition.SourceStateKey],
				TargetStateKey:  transition.TargetStateKey,
				TargetStateName: targetStateMap[transition.TargetStateKey],
				TransitionID:    transition.TransitionID,
			})
		}
	}

	return &MeegoTransitionResp{TargetStatus: availableTransition}, nil
}
