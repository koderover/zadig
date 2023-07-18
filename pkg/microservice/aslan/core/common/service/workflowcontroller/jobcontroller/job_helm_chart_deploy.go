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

package jobcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
)

type HelmChartDeployJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	restConfig  *rest.Config
	jobTaskSpec *commonmodels.JobTaskHelmChartDeploySpec
	ack         func()
}

func NewHelmChartDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *HelmChartDeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskHelmChartDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &HelmChartDeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *HelmChartDeployJobCtl) Clean(ctx context.Context) {}

func (c *HelmChartDeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project %s error: %v", c.workflowCtx.ProjectName, err)
		logError(c.job, msg, c.logger)
		return
	}
	c.namespace = productInfo.Namespace
	c.jobTaskSpec.ClusterID = productInfo.ClusterID

	renderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		Name:        productInfo.Render.Name,
		EnvName:     productInfo.EnvName,
		Revision:    productInfo.Render.Revision,
	})
	if err != nil {
		err = fmt.Errorf("failed to find redset name %s revision %d", productInfo.Namespace, productInfo.Render.Revision)
		logError(c.job, err.Error(), c.logger)
		return
	}

	for _, deploy := range c.jobTaskSpec.DeployHelmCharts {
		chartInfo, ok := renderSet.GetChartRenderMap()[deploy.ReleaseName]
		if !ok {
			msg := fmt.Sprintf("failed to find chart info in render")
			logError(c.job, msg, c.logger)
			return
		}
		if chartInfo.OverrideYaml == nil {
			chartInfo.OverrideYaml = &template.CustomYaml{}
		}

		valuesYaml := deploy.ValuesYaml
		chartInfo.OverrideYaml.YamlContent = valuesYaml
		c.ack()

		c.logger.Infof("start helm chart deploy, productName %s, releaseName %s, namespace %s, variableYaml %s, overrideValues: %s",
			c.workflowCtx.ProjectName, deploy.ReleaseName, c.namespace, valuesYaml, chartInfo.OverrideValues)

		timeOut := c.timeout()

		param := &kube.HelmChartInstallParam{
			ProductName:  c.workflowCtx.ProjectName,
			EnvName:      c.jobTaskSpec.Env,
			ReleaseName:  deploy.ReleaseName,
			ChartRepo:    deploy.ChartRepo,
			ChartName:    deploy.ChartName,
			ChartVersion: deploy.ChartVersion,
			VariableYaml: valuesYaml,
		}

		valuesMap := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(valuesYaml), &valuesMap)
		if err != nil {
			logError(c.job, fmt.Sprintf("Failed to unmarshall yaml, err %s", err), c.logger)
		}
		containerList, err := commonutil.ParseImagesForProductService(valuesMap, deploy.ReleaseName, c.workflowCtx.ProjectName)
		if err != nil {
			logError(c.job, fmt.Sprintf("Failed to parse container from yaml, err %s", err), c.logger)
		}
		productChartService := productInfo.GetChartServiceMap()[deploy.ReleaseName]
		if productChartService == nil {
			productChartService = &commonmodels.ProductService{
				ReleaseName: deploy.ReleaseName,
				ProductName: c.workflowCtx.ProjectName,
				Type:        setting.HelmChartDeployType,
			}
		}
		productChartService.Containers = containerList

		done := make(chan bool)
		go func(chan bool) {
			if err = kube.UpgradeHelmChartRelease(productInfo, renderSet, productChartService, param, timeOut); err != nil {
				err = errors.WithMessagef(
					err,
					"failed to upgrade helm chart %s/%s",
					c.namespace, deploy.ReleaseName)
				done <- false
			} else {
				done <- true
			}
		}(done)

		// we add timeout check here in case helm stuck in pending status
		select {
		case result := <-done:
			if !result {
				logError(c.job, err.Error(), c.logger)
				return
			}
			break
		case <-time.After(time.Second*time.Duration(timeOut) + time.Minute):
			err = fmt.Errorf("failed to upgrade relase for service: %s, timeout", deploy.ReleaseName)
		}
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return
		}
	}

	c.job.Status = config.StatusPassed
}

func (c *HelmChartDeployJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}

func (c *HelmChartDeployJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),

		ServiceType: setting.HelmDeployType,
		TargetEnv:   c.jobTaskSpec.Env,
		Production:  true,
	})
}
