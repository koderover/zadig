/*
Copyright 2023 The KodeRover Authors.

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

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateStatDashboardConfig(args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
	}

	if args.APIConfig != nil {
		config.APIConfig = &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		}
	}

	err := commonrepo.NewStatDashboardConfigColl().Create(context.TODO(), config)
	if err != nil {
		logger.Errorf("failed to create config for type: %s, error: %s", args.Type, err)
		return e.ErrCreateStatisticsDashboardConfig.AddDesc(err.Error())
	}
	return nil
}

func ListDashboardConfigs(logger *zap.SugaredLogger) ([]*StatDashboardConfig, error) {
	configs, err := commonrepo.NewStatDashboardConfigColl().List()
	if err != nil {
		logger.Errorf("failed to list dashboard configs, error: %s", err)
		return nil, e.ErrListStatisticsDashboardConfig.AddDesc(err.Error())
	}

	if len(configs) == 0 {
		err := initializeStatDashboardConfig()
		if err != nil {
			logger.Errorf("failed to initialize dashboard configs, error: %s", err)
			return nil, e.ErrListStatisticsDashboardConfig.AddDesc(err.Error())
		}
		configs = createDefaultStatDashboardConfig()
	}

	var result []*StatDashboardConfig
	for _, config := range configs {
		currentResult := &StatDashboardConfig{
			ID:       config.ItemKey,
			Type:     config.Type,
			Name:     config.Name,
			Source:   config.Source,
			Function: config.Function,
			Weight:   config.Weight,
		}
		if config.APIConfig != nil {
			currentResult.APIConfig = &APIConfig{
				ExternalSystemId: config.APIConfig.ExternalSystemId,
				ApiPath:          config.APIConfig.ApiPath,
				Queries:          config.APIConfig.Queries,
			}
		}
		result = append(result, currentResult)
	}
	return result, nil
}

func UpdateStatDashboardConfig(id string, args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
	}

	if args.APIConfig != nil {
		config.APIConfig = &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		}
	}

	err := commonrepo.NewStatDashboardConfigColl().Update(context.TODO(), id, config)
	if err != nil {
		logger.Errorf("failed to update config for type: %s, error: %s", args.Type, err)
	}
	return e.ErrUpdateStatisticsDashboardConfig.AddDesc(err.Error())
}

func DeleteStatDashboardConfig(id string, logger *zap.SugaredLogger) error {
	err := commonrepo.NewStatDashboardConfigColl().Delete(context.TODO(), id)
	if err != nil {
		logger.Errorf("failed to delete config for id: %s, error: %s", id, err)
		e.ErrDeleteStatisticsDashboardConfig.AddDesc(err.Error())
	}
	return nil
}

func GetStatsDashboard(startTime, endTime int64, logger *zap.SugaredLogger) ([]*StatDashboardByProject, error) {
	resp := make([]*StatDashboardByProject, 0)

	configs, err := commonrepo.NewStatDashboardConfigColl().List()
	if err != nil {
		logger.Errorf("failed to list dashboard configs, error: %s", err)
		return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
	}

	if len(configs) == 0 {
		err := initializeStatDashboardConfig()
		if err != nil {
			logger.Errorf("failed to initialize dashboard configs, error: %s", err)
			return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
		}
		configs = createDefaultStatDashboardConfig()
	}

	projects, err := templaterepo.NewProductColl().ListNonPMProject()
	if err != nil {
		logger.Errorf("failed to list projects to create dashborad, error: %s", err)
		return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
	}

	for _, project := range projects {
		facts := make([]*StatDashboardItem, 0)

		for _, config := range configs {
			cfg := &StatDashboardConfig{
				ID:       config.ItemKey,
				Type:     config.Type,
				Name:     config.Name,
				Source:   config.Source,
				Function: config.Function,
				Weight:   config.Weight,
			}
			if config.APIConfig != nil {
				cfg.APIConfig = &APIConfig{
					ExternalSystemId: config.APIConfig.ExternalSystemId,
					ApiPath:          config.APIConfig.ApiPath,
					Queries:          config.APIConfig.Queries,
				}
			}
			calculator, err := CreateCalculatorFromConfig(cfg)
			if err != nil {
				logger.Errorf("failed to create calculator for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				// if for some reason we failed to create the calculator, we append a fact with value 0, and error along with it
				facts = append(facts, &StatDashboardItem{
					Type:  config.Type,
					ID:    config.ItemKey,
					Data:  0,
					Score: 0,
					Error: err.Error(),
				})
				continue
			}
			fact, err := calculator.GetFact(startTime, endTime, project.Name)
			if err != nil {
				logger.Errorf("failed to get fact for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				// if for some reason we failed to get the fact, we append a fact with value 0, and error along with it
				facts = append(facts, &StatDashboardItem{
					Type:  config.Type,
					ID:    config.ItemKey,
					Data:  0,
					Score: 0,
					Error: err.Error(),
				})
				continue
			}
			// otherwise we calculate the score and append the fact
			score, err := calculator.GetWeightedScore(fact)
			if err != nil {
				logger.Errorf("failed to calculate score for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				score = 0
			}
			facts = append(facts, &StatDashboardItem{
				Type:  config.Type,
				ID:    config.ItemKey,
				Data:  fact,
				Score: score,
				Error: err.Error(),
			})
		}

		// once all configured facts are calculated, we calculate the total score
		totalScore := 0.0
		for _, fact := range facts {
			totalScore += fact.Score
		}

		resp = append(resp, &StatDashboardByProject{
			ProjectKey:  project.Name,
			ProjectName: project.Alias,
			Score:       totalScore,
			Facts:       facts,
		})
	}
	return resp, nil
}

var defaultStatDashboardConfigMap map[string]*commonmodels.StatDashboardConfig

func createDefaultStatDashboardConfig() []*commonmodels.StatDashboardConfig {
	ret := make([]*commonmodels.StatDashboardConfig, 0)
	for _, cfg := range defaultStatDashboardConfigMap {
		ret = append(ret, cfg)
	}
	return ret
}

func initializeStatDashboardConfig() error {
	return commonrepo.NewStatDashboardConfigColl().BulkCreate(context.TODO(), createDefaultStatDashboardConfig())
}
