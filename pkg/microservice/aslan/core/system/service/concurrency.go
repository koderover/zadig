package service

import (
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func GetWorkflowConcurrency() (*WorkflowConcurrencySettings, error) {
	setting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		return nil, err
	}
	return &WorkflowConcurrencySettings{
		WorkflowConcurrency: setting.WorkflowConcurrency,
		BuildConcurrency:    setting.BuildConcurrency,
	}, nil
}
