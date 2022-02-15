package service

import (
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
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

func UpdateWorkflowConcurrency(workflowConcurrency, buildConcurrency int64, log *zap.SugaredLogger) error {
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
