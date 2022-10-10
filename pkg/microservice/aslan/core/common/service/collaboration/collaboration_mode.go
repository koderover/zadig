/*
Copyright 2022 The KodeRover Authors.

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

package collaboration

import (
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

type GetCollaborationModeResp struct {
	Collaborations []*models.CollaborationMode `json:"collaborations"`
}

func BuildEnvCMMapKey(projectName string, envName string) string {
	return projectName + " " + envName
}

func BuildWorkflowCMMapKey(projectName string, workflowName string) string {
	return projectName + " " + workflowName
}

func GetEnvCMMap(projects []string, logger *zap.SugaredLogger) (map[string]sets.String, error) {
	collaborationModes, err := GetCollaborationModes(projects, logger)
	if err != nil {
		logger.Errorf("GetCollaborationModes error: %v", err)
		return nil, err
	}
	return buildEnvCMMap(collaborationModes.Collaborations), nil
}

func GetWorkflowCMMap(projects []string, logger *zap.SugaredLogger) (map[string]sets.String, error) {
	collaborationModes, err := GetCollaborationModes(projects, logger)
	if err != nil {
		logger.Errorf("GetCollaborationModes error: %v", err)
		return nil, err
	}
	return buildWorkflowCMMap(collaborationModes.Collaborations), nil
}

func buildEnvCMMap(collaborations []*models.CollaborationMode) map[string]sets.String {
	envCMMap := make(map[string]sets.String)
	for _, cm := range collaborations {
		for _, product := range cm.Products {
			key := BuildEnvCMMapKey(cm.ProjectName, product.Name)
			if cmSet, ok := envCMMap[key]; ok {
				cmSet.Insert(cm.Name)
				envCMMap[key] = cmSet
			} else {
				cmSet := sets.NewString(cm.Name)
				envCMMap[key] = cmSet
			}
		}
	}
	return envCMMap
}

func buildWorkflowCMMap(collaborations []*models.CollaborationMode) map[string]sets.String {
	workflowCMMap := make(map[string]sets.String)
	for _, cm := range collaborations {
		for _, workflow := range cm.Workflows {
			key := BuildWorkflowCMMapKey(cm.ProjectName, workflow.Name)
			if cmSet, ok := workflowCMMap[key]; ok {
				cmSet.Insert(cm.Name)
				workflowCMMap[key] = cmSet
			} else {
				cmSet := sets.NewString(cm.Name)
				workflowCMMap[key] = cmSet
			}
		}
	}
	return workflowCMMap
}

func GetCollaborationModes(projects []string, logger *zap.SugaredLogger) (*GetCollaborationModeResp, error) {
	collaborations, err := mongodb.NewCollaborationModeColl().List(&mongodb.CollaborationModeListOptions{
		Projects: projects,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &GetCollaborationModeResp{}, nil
		}
		logger.Errorf("GetCollaborationModes error, err msg:%s", err)
		return nil, err
	}
	for _, mode := range collaborations {
		setCollaborationModesWorkflowDisplayName(mode)
	}
	return &GetCollaborationModeResp{
		Collaborations: collaborations,
	}, nil
}

func setCollaborationModesWorkflowDisplayName(mode *models.CollaborationMode) {
	names := []string{}
	v4Names := []string{}
	for _, workflow := range mode.Workflows {
		if workflow.WorkflowType == "common_workflow" {
			v4Names = append(v4Names, workflow.Name)
			continue
		}
		names = append(names, workflow.Name)
	}
	namesMap := map[string]string{}
	v4NamesMap := map[string]string{}
	if len(names) > 0 {
		workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{mode.ProjectName}, Names: names})
		if err != nil {
			log.Errorf("list workflow in project %s error: %v", mode.ProjectName, err)
		}
		for _, workflow := range workflows {
			namesMap[workflow.Name] = workflow.DisplayName
		}
	}
	if len(v4Names) > 0 {
		workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{ProjectName: mode.ProjectName, Names: v4Names}, 0, 0)
		if err != nil {
			log.Errorf("list workflow v4 in project %s error: %v", mode.ProjectName, err)
		}
		for _, workflow := range workflows {
			v4NamesMap[workflow.Name] = workflow.DisplayName
		}
	}
	for i, workflow := range mode.Workflows {
		if workflow.WorkflowType == "common_workflow" {
			mode.Workflows[i].DisplayName = v4NamesMap[workflow.Name]
			continue
		}
		mode.Workflows[i].DisplayName = namesMap[workflow.Name]
	}
}
