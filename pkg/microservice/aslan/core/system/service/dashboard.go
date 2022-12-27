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

package service

import (
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"math"
)

const (
	CardNameRunningWorkflow        = "运行中的工作流"
	CardNameServiceUpdateFrequency = "服务热力图"
)

const (
	CardTypeRunningWorkflow        = "running_workflow"
	CardTypeServiceUpdateFrequency = "service_update_frequency"
	CardTypeMyWorkflow             = "my_workflow"
	CardTypeMyEnv                  = "my_env"
)

func CreateOrUpdateDashboardConfiguration(username, userID string, config *DashBoardConfig, log *zap.SugaredLogger) error {
	cardConfig := make([]*commonmodels.CardConfig, 0)
	for _, cfg := range config.Cards {
		cardConfig = append(cardConfig, &commonmodels.CardConfig{
			ID:     cfg.ID,
			Name:   cfg.Name,
			Type:   cfg.Type,
			Config: cfg.Config,
		})
	}
	dashboardConfig := &commonmodels.DashboardConfig{
		Cards:    cardConfig,
		UserID:   userID,
		UserName: username,
	}

	return commonrepo.NewDashboardConfigColl().CreateOrUpdate(dashboardConfig)
}

func GetDashboardConfiguration(username, userID string, log *zap.SugaredLogger) (*DashBoardConfig, error) {
	cfg, err := commonrepo.NewDashboardConfigColl().GetByUser(username, userID)
	// if there is an error and the error is not empty document then we return error
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, err
		} else {
			return generateDefaultDashboardConfig(), nil
		}
	}
	cardConfig := make([]*DashBoardCardConfig, 0)
	for _, card := range cfg.Cards {
		retConfig := &DashBoardCardConfig{
			ID:     card.ID,
			Name:   card.Name,
			Type:   card.Type,
			Config: card.Config,
		}
		cardConfig = append(cardConfig, retConfig)
	}
	return &DashBoardConfig{Cards: cardConfig}, nil
}

func GetRunningWorkflow(log *zap.SugaredLogger) ([]*WorkflowResponse, error) {
	resp := make([]*WorkflowResponse, 0)
	runningQueue := workflow.RunningTasks()
	pendingQueue := workflow.PendingTasks()
	for _, runningtask := range runningQueue {
		resp = append(resp, &WorkflowResponse{
			Name:        runningtask.PipelineName,
			Project:     runningtask.ProductName,
			Creator:     runningtask.TaskCreator,
			StartTime:   runningtask.StartTime,
			Status:      string(runningtask.Status),
			DisplayName: runningtask.PipelineDisplayName,
			Type:        string(runningtask.Type),
		})
	}
	for _, pendingTask := range pendingQueue {
		resp = append(resp, &WorkflowResponse{
			Name:        pendingTask.PipelineName,
			Project:     pendingTask.ProductName,
			Creator:     pendingTask.TaskCreator,
			StartTime:   pendingTask.StartTime,
			Status:      string(pendingTask.Status),
			DisplayName: pendingTask.PipelineDisplayName,
			Type:        string(pendingTask.Type),
		})
	}

	return resp, nil
}

func GetMyWorkflow(username, userID string, log *zap.SugaredLogger) (*WorkflowCardResponse, error) {
	cfg, err := commonrepo.NewDashboardConfigColl().GetByUser(username, userID)
	// if there is an error and the error is not empty document then we return error
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, err
		} else {
			// if no config is found, then no my workflow is configured, return empty
			return &WorkflowCardResponse{Cards: make([]*WorkflowCard, 0)}, nil
		}
	}

	cardList := make([]*WorkflowCard, 0)

	for _, cardCfg := range cfg.Cards {
		if cardCfg.Type == CardTypeMyWorkflow {
			sampleWorkflowList := make([]*WorkflowResponse, 0)
			sampleWorkflowList = append(sampleWorkflowList, &WorkflowResponse{
				Name:        "test-pending-custom-workflow",
				Project:     "test-project",
				Creator:     "minmin",
				StartTime:   1672043731,
				Status:      "pending",
				DisplayName: "测试用pending自定义工作流",
				Type:        "common_workflow",
			})
			cardList = append(cardList, &WorkflowCard{
				CardID:    cardCfg.ID,
				Workflows: sampleWorkflowList,
			})
		}
	}
	return &WorkflowCardResponse{Cards: cardList}, nil
}

const (
	DefaultEnvFilter = "serviceName=*,name="
)

func GetMyEnvironment(projectName, envName, username, userID string, log *zap.SugaredLogger) (*EnvResponse, error) {
	cfg, err := commonrepo.NewDashboardConfigColl().GetByUser(username, userID)
	// if there is an error and the error is not empty document then we return error
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, err
		} else {
			// if no config is found, then no my env is configured, return empty
			return nil, nil
		}
	}
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    projectName,
		EnvName: envName,
	})
	if err != nil {
		return nil, err
	}
	serviceList := make([]*EnvService, 0)
	_, svcList, err := service.ListWorkloadsInEnv(envName, projectName, DefaultEnvFilter, math.MaxInt, 1, log)
	if err != nil {
		log.Errorf("failed to get workloads in the env, error: %s", err)
		return nil, err
	}
	targetServiceMap := make(map[string]int)
	for _, card := range cfg.Cards {
		if card.Type == CardTypeMyEnv {
			envConfig := card.Config.(MyEnvCardConfig)
			if envConfig.EnvName == envName && envConfig.ProjectName == projectName {
				for _, svc := range envConfig.ServiceModules {
					targetServiceMap[svc] = 1
				}
			}
		}
	}
	for _, svc := range svcList {
		if _, ok := targetServiceMap[svc.ServiceName]; ok {
			serviceList = append(serviceList, &EnvService{
				ServiceName: svc.ServiceDisplayName,
				Status:      svc.Status,
				Image:       svc.Images[0],
			})
		}
	}
	return &EnvResponse{
		Name:        envName,
		ProjectName: projectName,
		UpdateTime:  productInfo.UpdateTime,
		UpdatedBy:   productInfo.UpdateBy,
		Services:    serviceList,
	}, nil
}

func generateDefaultDashboardConfig() *DashBoardConfig {
	cardConfig := make([]*DashBoardCardConfig, 0)
	cardConfig = append(cardConfig, &DashBoardCardConfig{
		Name: CardNameRunningWorkflow,
		Type: CardTypeRunningWorkflow,
	})
	cardConfig = append(cardConfig, &DashBoardCardConfig{
		Name: CardNameServiceUpdateFrequency,
		Type: CardTypeServiceUpdateFrequency,
	})
	return &DashBoardConfig{Cards: cardConfig}
}
