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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"go.uber.org/zap"
)

func CreateStatDashboardConfig(args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
		APIConfig: &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		},
	}

	err := commonrepo.NewStatDashboardConfigColl().Create(context.TODO(), config)
	if err != nil {
		logger.Errorf("failed to create config for type: %s, error: %s", args.Type, err)
	}
	return e.ErrCreateStatisticsDashboardConfig.AddDesc(err.Error())
}

func ListDashboardConfigs(logger *zap.SugaredLogger) ([]*StatDashboardConfig, error) {
	configs, err := commonrepo.NewStatDashboardConfigColl().List(context.TODO())
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
		result = append(result, &StatDashboardConfig{
			ID:       config.ItemKey,
			Type:     config.Type,
			Name:     config.Name,
			Source:   config.Source,
			Function: config.Function,
			Weight:   config.Weight,
			APIConfig: &APIConfig{
				ExternalSystemId: config.APIConfig.ExternalSystemId,
				ApiPath:          config.APIConfig.ApiPath,
				Queries:          config.APIConfig.Queries,
			},
		})
	}
	return result, nil
}

func UpdateStatDashboardConfig(args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
		APIConfig: &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		},
	}

	err := commonrepo.NewStatDashboardConfigColl().Update(context.TODO(), args.ID, config)
	if err != nil {
		logger.Errorf("failed to update config for type: %s, error: %s", args.Type, err)
	}
	return e.ErrUpdateStatisticsDashboardConfig.AddDesc(err.Error())
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
