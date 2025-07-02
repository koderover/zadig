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
	"errors"

	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
)

func GetWorkflowConcurrency() (*WorkflowConcurrencySettings, error) {
	configuration, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		return nil, err
	}
	return &WorkflowConcurrencySettings{
		WorkflowConcurrency: configuration.WorkflowConcurrency,
		BuildConcurrency:    configuration.BuildConcurrency,
	}, nil
}

func UpdateWorkflowConcurrency(workflowConcurrency, buildConcurrency int64, log *zap.SugaredLogger) error {
	// check if there are running tasks
	tasks := workflowcontroller.RunningTasks()
	if len(tasks) > 0 {
		return errors.New("workflow settings must be set when NO task is running")
	}
	// first update system configuration table
	err := commonrepo.NewSystemSettingColl().UpdateConcurrencySetting(workflowConcurrency, buildConcurrency)
	if err != nil {
		log.Errorf("Failed to update system settings, the error is: %s", err)
		return err
	}

	return nil
}
