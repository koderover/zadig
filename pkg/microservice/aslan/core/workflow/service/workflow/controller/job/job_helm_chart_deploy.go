/*
Copyright 2025 The KodeRover Authors.

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

package job

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type HelmChartDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigHelmChartDeployJobSpec
}

func CreateHelmChartDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigHelmChartDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return HelmChartDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j HelmChartDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j HelmChartDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j HelmChartDeployJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigHelmChartDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if isExecution {
		if j.jobSpec.Env != currJobSpec.Env && currJobSpec.EnvSource == "fixed" {
			return fmt.Errorf("env is set to fixed [%s] in workflow settings", currJobSpec.Env)
		}
	}

	return nil
}

func (j HelmChartDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigHelmChartDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if !useUserInput {
		product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.jobSpec.Env})
		if err != nil {
			return fmt.Errorf("env %s not exists", j.jobSpec.Env)
		}
		renderChartMap := product.GetChartDeployRenderMap()

		deploys := []*commonmodels.DeployHelmChart{}
		productChartServiceMap := product.GetChartServiceMap()
		for _, chartSvc := range productChartServiceMap {
			renderChart := renderChartMap[chartSvc.ReleaseName]
			if renderChart == nil {
				return fmt.Errorf("render chart %s not found", chartSvc.ReleaseName)
			}
			deploy := &commonmodels.DeployHelmChart{
				ReleaseName:  chartSvc.ReleaseName,
				ChartRepo:    renderChart.ChartRepo,
				ChartName:    renderChart.ChartName,
				ChartVersion: renderChart.ChartVersion,
				ValuesYaml:   renderChart.GetOverrideYaml(),
				OverrideKVs:  renderChart.OverrideValues,
			}
			deploys = append(deploys, deploy)
		}
		j.jobSpec.DeployHelmCharts = deploys
	}

	j.jobSpec.EnvSource = currJobSpec.EnvSource
	j.jobSpec.SkipCheckRunStatus = currJobSpec.SkipCheckRunStatus
	return nil
}

func (j HelmChartDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigHelmChartDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	envOptions := make([]*commonmodels.ZadigHelmDeployEnvInformation, 0)

	if currJobSpec.EnvSource == "fixed" {
		if ticket.IsAllowedEnv(j.workflow.Project, currJobSpec.Env) {
			chartInfo, err := generateEnvHelmChartInfo(currJobSpec.Env, j.workflow.Project)
			if err != nil {
				return err
			}

			envOptions = append(envOptions, &commonmodels.ZadigHelmDeployEnvInformation{
				Env:      currJobSpec.Env,
				Services: chartInfo,
			})
		}
	} else {
		productList, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: &j.jobSpec.Production,
		})
		if err != nil {
			return fmt.Errorf("failed to list env with project: %s, error: %s", j.workflow.Project, err)
		}

		for _, env := range productList {
			if !ticket.IsAllowedEnv(j.workflow.Project, env.EnvName) {
				continue
			}

			serviceDeployOption, err := generateEnvHelmChartInfo(env.EnvName, j.workflow.Project)
			if err != nil {
				return err
			}

			envOptions = append(envOptions, &commonmodels.ZadigHelmDeployEnvInformation{
				Env:      env.EnvName,
				Services: serviceDeployOption,
			})
		}
	}

	j.jobSpec.EnvOptions = envOptions
	return nil
}

func (j HelmChartDeployJobController) ClearOptions() {
	j.jobSpec.EnvOptions = nil
}

func (j HelmChartDeployJobController) ClearSelection() {
	j.jobSpec.DeployHelmCharts = make([]*commonmodels.DeployHelmChart, 0)
}

func (j HelmChartDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	envName := j.jobSpec.Env
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", envName)
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	if templateProduct.ProductFeature.DeployType != setting.HelmDeployType {
		return resp, fmt.Errorf("product %s deploy type is not helm", j.workflow.Project)
	}

	for subJobTaskID, deploy := range j.jobSpec.DeployHelmCharts {
		jobTaskSpec := &commonmodels.JobTaskHelmChartDeploySpec{
			Env:                envName,
			DeployHelmChart:    deploy,
			SkipCheckRunStatus: j.jobSpec.SkipCheckRunStatus,
			ClusterID:          product.ClusterID,
			Timeout:            timeout,
			MaxHistory:         templateProduct.ReleaseMaxHistory,
		}

		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, subJobTaskID),
			Key:         genJobKey(j.name),
			DisplayName: genJobDisplayName(j.name),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"release_name": deploy.ReleaseName,
			},
			JobType:       string(config.JobZadigHelmChartDeploy),
			Spec:          jobTaskSpec,
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j HelmChartDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j HelmChartDeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j HelmChartDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j HelmChartDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j HelmChartDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j HelmChartDeployJobController) IsServiceTypeJob() bool {
	return false
}

func generateEnvHelmChartInfo(env, project string) ([]*commonmodels.DeployHelmChart, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: project, EnvName: env})
	if err != nil {
		return nil, fmt.Errorf("failed to get env information from db, error: %s", err)
	}
	renderChartMap := product.GetChartDeployRenderMap()

	deploys := make([]*commonmodels.DeployHelmChart, 0)
	productChartServiceMap := product.GetChartServiceMap()
	for _, chartSvc := range productChartServiceMap {
		renderChart := renderChartMap[chartSvc.ReleaseName]
		if renderChart == nil {
			return nil, fmt.Errorf("failed to get service render info for service: %s in env: %s", chartSvc.ServiceName, env)
		}
		deploy := &commonmodels.DeployHelmChart{
			ReleaseName:  chartSvc.ReleaseName,
			ChartRepo:    renderChart.ChartRepo,
			ChartName:    renderChart.ChartName,
			ChartVersion: renderChart.ChartVersion,
			ValuesYaml:   renderChart.GetOverrideYaml(),
			OverrideKVs:  renderChart.OverrideValues,
		}
		deploys = append(deploys, deploy)
	}

	return deploys, nil
}
