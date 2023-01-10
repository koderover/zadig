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
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/meego"
)

func GetMeegoProjects() (*MeegoProjectResp, error) {
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeego()
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := meego.NewClient(meegoInfo.MeegoHost, meegoInfo.MeegoPluginID, meegoInfo.MeegoPluginSecret, meegoInfo.MeegoUserKey)
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

func GetWorkItemTypeList(projectID string) (*MeegoWorkItemTypeResp, error) {
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeego()
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := meego.NewClient(meegoInfo.MeegoHost, meegoInfo.MeegoPluginID, meegoInfo.MeegoPluginSecret, meegoInfo.MeegoUserKey)
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

func ListMeegoWorkItems(projectID, typeKey, nameQuery string, pageNum, pageSize int) (*MeegoWorkItemResp, error) {
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeego()
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := meego.NewClient(meegoInfo.MeegoHost, meegoInfo.MeegoPluginID, meegoInfo.MeegoPluginSecret, meegoInfo.MeegoUserKey)
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
			CurrentState: workItem.WorkItemStatus.StateKey,
		})
	}

	return &MeegoWorkItemResp{WorkItems: meegoWorkItemList}, nil
}

func ListAvailableWorkItemTransitions(projectID, typeKey string, workItemID int) (*MeegoTransitionResp, error) {
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeego()
	if err != nil {
		log.Errorf("failed to get meego info, err: %s", err)
		return nil, err
	}
	client, err := meego.NewClient(meegoInfo.MeegoHost, meegoInfo.MeegoPluginID, meegoInfo.MeegoPluginSecret, meegoInfo.MeegoUserKey)
	if err != nil {
		return nil, err
	}

	workItem, err := client.GetWorkItem(projectID, typeKey, workItemID)
	if err != nil {
		return nil, err
	}

	transitions, err := client.GetWorkFlowInfo(projectID, typeKey, workItemID)
	if err != nil {
		return nil, err
	}

	availableTransition := make([]*MeegoWorkItemStatusTransition, 0)
	for _, transition := range transitions {
		if workItem.WorkItemStatus.StateKey == transition.SourceStateKey {
			availableTransition = append(availableTransition, &MeegoWorkItemStatusTransition{
				SourceStateKey: transition.SourceStateKey,
				TargetStateKey: transition.TargetStateKey,
				TransitionID:   transition.TransitionID,
			})
		}
	}

	return &MeegoTransitionResp{TargetStatus: availableTransition}, nil
}
