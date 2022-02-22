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

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
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
	tasks := workflowservice.RunningPipelineTasks()
	if len(tasks) > 0 {
		return errors.New("workflow settings must be set when NO task is running")
	}
	// first update system configuration table
	err := commonrepo.NewSystemSettingColl().UpdateConcurrencySetting(workflowConcurrency, buildConcurrency)
	if err != nil {
		log.Errorf("Failed to update system settings, the error is: %s", err)
		return err
	}
	// then we update the warpdrive deployment, where the workflow concurrency happens
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}
	return updater.ScaleDeployment(config.Namespace(), configbase.WarpDriveServiceName(), int(workflowConcurrency), kubeClient)
}
