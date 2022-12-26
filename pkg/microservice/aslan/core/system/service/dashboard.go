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
	"context"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"time"
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

func CreateOrUpdateDashboardConfiguration(userName, userID string, config *DashBoardConfig, log *zap.SugaredLogger) error {
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
		UserName: userName,
	}

	return commonrepo.NewDashboardConfigColl().CreateOrUpdate(dashboardConfig)
}

func GetDashboardConfiguration(userName, userID string, log *zap.SugaredLogger) (*DashBoardConfig, error) {
	cfg, err := commonrepo.NewDashboardConfigColl().GetByUser(userName, userID)
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

func GetRunningWorkflow(ctx context.Context, streamChan chan interface{}, log *zap.SugaredLogger) {
	log.Infof("Starting to get information about running workflow")
	for {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			log.Infof("Connection is closed, container log stream stopped")
			return
		default:
			resp := make([]*WorkflowReponse, 0)
			resp = append(resp, &WorkflowReponse{
				Name:        "test-running-workflow",
				Project:     "test-project",
				Creator:     "minmin",
				StartTime:   1672043731,
				Status:      "running",
				DisplayName: "测试用工作流",
				Type:        "",
			})
			resp = append(resp, &WorkflowReponse{
				Name:        "test-pending-custom-workflow",
				Project:     "test-project",
				Creator:     "minmin",
				StartTime:   1672043731,
				Status:      "running",
				DisplayName: "测试用pending自定义工作流",
				Type:        "custom",
			})
			streamChan <- resp
		}
	}
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
