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

	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/koderover/zadig/v2/pkg/types/job"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	saeservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/sae"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type SAEDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.SAEDeployJobSpec
}

func CreateSAEDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.SAEDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return SAEDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j SAEDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j SAEDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j SAEDeployJobController) Validate(isExecution bool) error {
	if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j SAEDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.SAEDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.DockerRegistryID = currJobSpec.DockerRegistryID
	if currJobSpec.EnvConfig.Source == "fixed" {
		j.jobSpec.EnvConfig = currJobSpec.EnvConfig
	}
	j.jobSpec.Production = currJobSpec.Production
	j.jobSpec.JobName = currJobSpec.JobName
	j.jobSpec.OriginJobName = currJobSpec.OriginJobName
	j.jobSpec.ServiceConfig.Source = currJobSpec.ServiceConfig.Source

	return nil
}

func (j SAEDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	envOptions, err := generateSAEEnvOption(j.workflow.Project, ticket)
	if err != nil {
		return err
	}
	j.jobSpec.EnvOptions = envOptions

	return nil
}

func (j SAEDeployJobController) ClearOptions() {
	j.jobSpec.EnvConfig = nil
}

func (j SAEDeployJobController) ClearSelection() {
	return
}

func (j SAEDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	if j.jobSpec.ServiceConfig.Source == config.SourceFromJob {
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}

	}

	envInfo, err := commonrepo.NewSAEEnvColl().Find(&commonrepo.SAEEnvFindOptions{
		ProjectName:       j.workflow.Project,
		EnvName:           j.jobSpec.EnvConfig.Name,
		Production:        &j.jobSpec.Production,
		IgnoreNotFoundErr: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find env of name: %s, error: %s", j.jobSpec.EnvConfig.Name, err)
	}

	for jobSubTaskID, svc := range j.jobSpec.ServiceConfig.Services {
		if j.jobSpec.ServiceConfig.Source == config.SourceFromJob {
			svc.Image = job.GetJobOutputKey(fmt.Sprintf("%s.%s.%s", j.jobSpec.OriginJobName, svc.ServiceName, svc.ServiceModule), IMAGEKEY)
		}

		jobTaskSpec := &commonmodels.JobTaskSAEDeploySpec{
			Env:        j.jobSpec.EnvConfig.Name,
			Production: j.jobSpec.Production,

			AppID:         svc.AppID,
			AppName:       svc.AppName,
			ServiceName:   svc.ServiceName,
			ServiceModule: svc.ServiceModule,
			RegionID:      envInfo.RegionID,

			Image:                 svc.Image,
			UpdateStrategy:        svc.UpdateStrategy,
			BatchWaitTime:         svc.BatchWaitTime,
			MinReadyInstances:     svc.MinReadyInstances,
			MinReadyInstanceRatio: svc.MinReadyInstanceRatio,
			Envs:                  svc.Envs,
		}

		jobTask := &commonmodels.JobTask{
			Key:         genJobKey(j.name, svc.ServiceName),
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			DisplayName: genJobDisplayName(j.name, svc.ServiceName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"service_name": svc.ServiceName,
			},
			JobType:     string(config.JobSAEDeploy),
			Spec:        jobTaskSpec,
			ErrorPolicy: j.errorPolicy,
		}

		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j SAEDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j SAEDeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j SAEDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j SAEDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j SAEDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func generateSAEEnvOption(projectKey string, approvalTicket *commonmodels.ApprovalTicket) (envOptions []*commonmodels.SAEEnvInfo, err error) {
	saeModel, err := commonrepo.NewSAEColl().FindDefault()
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		return nil, err
	}

	envOptions = make([]*commonmodels.SAEEnvInfo, 0)

	envs, err := commonrepo.NewSAEEnvColl().List(&commonrepo.SAEEnvListOptions{
		ProjectName: projectKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sae envs for project: %s, error: %s", projectKey, err)
	}

	for _, env := range envs {
		if approvalTicket.IsAllowedEnv(projectKey, env.EnvName) {
			continue
		}

		serviceList := make([]*commonmodels.SAEServiceInfo, 0)

		saeClient, err := saeservice.NewClient(saeModel, env.RegionID)
		if err != nil {
			err = fmt.Errorf("failed to create sae client, err: %s", err)
			return nil, err
		}

		tags := fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, projectKey, setting.SAEZadigEnvTagKey, env.EnvName)

		saeRequest := &sae.ListApplicationsRequest{
			Tags:        tea.String(tags),
			CurrentPage: tea.Int32(1),
			// TODO: possibly fix the hard-coded paging.
			PageSize:    tea.Int32(10000),
			NamespaceId: tea.String(env.NamespaceID),
		}

		saeResp, err := saeClient.ListApplications(saeRequest)
		if err != nil {
			err = fmt.Errorf("failed to list applications, err: %s", err)
			return nil, err
		}

		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("failed to list applications, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			return nil, err
		}

		for _, saeApp := range saeResp.Body.Data.Applications {
			// if the sae app has not been tagged with the service name and service module, we ignore it
			tagged := false
			serviceName := ""
			serviceModule := ""
			for _, tag := range saeApp.Tags {
				if tea.StringValue(tag.Key) == setting.SAEZadigServiceTagKey {
					tagged = true
					serviceName = tea.StringValue(tag.Value)
				}

				if tea.StringValue(tag.Key) == setting.SAEZadigServiceModuleTagKey {
					serviceModule = tea.StringValue(tag.Value)
				}

			}

			if !tagged {
				continue
			}

			if !approvalTicket.IsAllowedService(projectKey, serviceName, serviceModule) {
				continue
			}

			describeAppReq := &sae.DescribeApplicationConfigRequest{
				AppId: saeApp.AppId,
			}

			appDetailResp, err := saeClient.DescribeApplicationConfig(describeAppReq)
			if err != nil {
				err = fmt.Errorf("failed to list applications, err: %s", err)
				return nil, err
			}

			if !tea.BoolValue(appDetailResp.Body.Success) {
				err = fmt.Errorf("failed to describe application, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
				return nil, err
			}

			kv := make([]*commonmodels.SAEKV, 0)

			saeKVMap, err := saeservice.CreateKVMap(appDetailResp.Body.Data.Envs)
			if err != nil {
				err = fmt.Errorf("failed to decode sae app's env variables, error: %s", err)
				return nil, err
			}

			for _, saeKV := range saeKVMap {
				kv = append(kv, saeKV)
			}

			serviceList = append(serviceList, &commonmodels.SAEServiceInfo{
				AppID:         tea.StringValue(saeApp.AppId),
				AppName:       tea.StringValue(saeApp.AppName),
				Image:         tea.StringValue(saeApp.ImageUrl),
				Instances:     tea.Int32Value(saeApp.Instances),
				Envs:          kv,
				ServiceName:   serviceName,
				ServiceModule: serviceModule,
			})
		}

		envOptions = append(envOptions, &commonmodels.SAEEnvInfo{
			Env:      env.EnvName,
			Services: serviceList,
		})
	}

	return envOptions, nil
}
