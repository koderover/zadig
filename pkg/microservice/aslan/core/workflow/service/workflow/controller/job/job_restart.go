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
	aslanUtil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	"k8s.io/apimachinery/pkg/util/sets"
)

type RestartJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigRestartJobSpec
}

func CreateRestartJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigRestartJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create build job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return RestartJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j RestartJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j RestartJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j RestartJobController) Validate(isExecution bool) error {
	if err := aslanUtil.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("重启任务需要专业版才能使用")
	}

	jobRankMap := GetJobRankMap(j.workflow.Stages)

	if j.jobSpec.Source == config.SourceFromJob {
		fromJobRank, ok := jobRankMap[j.jobSpec.JobName]
		if !ok || fromJobRank >= jobRankMap[j.name] {
			return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.JobName, j.name)
		}
		return nil
	}

	if j.jobSpec.EnvSource == config.SourceFromJob {
		envJobRank, ok := jobRankMap[j.jobSpec.EnvJobName]
		if !ok || envJobRank >= jobRankMap[j.name] {
			return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.EnvJobName, j.name)
		}
	}

	if isExecution {
		latestJob, err := j.workflow.FindJob(j.name, j.jobType)
		if err != nil {
			return fmt.Errorf("failed to find job: %s in workflow %s's latest config, error: %s", j.name, j.workflow.Name, err)
		}

		currJobSpec := new(commonmodels.ZadigRestartJobSpec)
		if err := commonmodels.IToi(latestJob.Spec, currJobSpec); err != nil {
			return fmt.Errorf("failed to decode zadig deploy job spec, error: %s", err)
		}

		if j.jobSpec.Env != currJobSpec.Env && currJobSpec.EnvSource == config.ParamSourceFixed {
			return fmt.Errorf("job %s cannot deploy to env: %s, configured env is fixed to %s", j.name, j.jobSpec.Env, currJobSpec.Env)
		}
	}
	return nil
}

func (j RestartJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	latestSpec := new(commonmodels.ZadigRestartJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.errorPolicy = latestJob.ErrorPolicy
	j.executePolicy = latestJob.ExecutePolicy

	j.jobSpec.Production = latestSpec.Production
	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if project.ProductFeature != nil {
		j.jobSpec.DeployType = project.ProductFeature.DeployType
	}
	j.jobSpec.SkipCheckRunStatus = latestSpec.SkipCheckRunStatus
	j.jobSpec.EnvSource = latestSpec.EnvSource

	// handle env first
	if j.jobSpec.EnvSource == config.SourceFromJob {
		j.jobSpec.EnvJobName = latestSpec.EnvJobName
		envFromjob, err := j.workflow.FindJob(j.jobSpec.EnvJobName, config.JobZadigDeploy)
		if err != nil {
			return fmt.Errorf("failed find env from job %s, err: %v", j.jobSpec.EnvJobName, err)
		}

		envFromjobSpec := new(commonmodels.ZadigDeployJobSpec)
		if err := commonmodels.IToi(envFromjob.Spec, envFromjobSpec); err != nil {
			return fmt.Errorf("failed to decode env from job spec, error: %s", err)
		}

		j.jobSpec.Env = envFromjobSpec.Env
	} else if j.jobSpec.Env != latestSpec.Env && latestSpec.EnvSource == config.SourceFixed {
		j.jobSpec.Env = latestSpec.Env
		j.jobSpec.Services = make([]string, 0)
		return nil
	} else {
		j.jobSpec.Env = latestSpec.Env
	}

	if !ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
		j.jobSpec.Env = ""
	}

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       j.workflow.Project,
		Production: util.GetBoolPointer(j.jobSpec.Production),
	})
	if err != nil {
		log.Errorf("can't list envs in project %s, error: %w", j.workflow.Project, err)
		return err
	}

	currentEnvMap := make(map[string]*commonmodels.Product)
	for _, env := range products {
		currentEnvMap[env.EnvName] = env
	}

	if _, ok := currentEnvMap[j.jobSpec.Env]; !ok {
		j.jobSpec.Env = ""
	}

	// if unselected for some reason, we skip calculating default service
	if j.jobSpec.Env == "" {
		j.jobSpec.Services = make([]string, 0)
		return nil
	}

	// hanlde service second
	// source is a bit tricky: if the saved args has a source of fromjob, but it has been change to runtime in the config
	// we need to not only update its source but also set services to empty slice.
	if j.jobSpec.Source == config.SourceFromJob && latestSpec.Source == config.SourceRuntime {
		j.jobSpec.Services = make([]string, 0)
	}
	j.jobSpec.Source = latestSpec.Source

	if j.jobSpec.Source == config.SourceFromJob {
		j.jobSpec.JobName = latestSpec.JobName
	}

	currentEnv := currentEnvMap[j.jobSpec.Env]
	newServices := make([]string, 0)
	// filter the services that are not in the env
	for _, service := range j.jobSpec.Services {
		if _, ok := currentEnv.GetServiceMap()[service]; ok {
			newServices = append(newServices, service)
		}
	}
	j.jobSpec.Services = newServices

	return nil
}

// SetOptions sets options for all the possible envs, there is a special case:
// it will update the env's option/ service's option based on the user's setting on whether they update the config
func (j RestartJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	envOptions := make([]*commonmodels.ZadigRestartEnvInformation, 0)

	if j.jobSpec.EnvSource == config.ParamSourceFixed {
		if !ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
			return fmt.Errorf("env %s is not allowed", j.jobSpec.Env)
		}

		env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:       j.workflow.Project,
			EnvName:    j.jobSpec.Env,
			Production: util.GetBoolPointer(j.jobSpec.Production),
		})
		if err != nil {
			err = fmt.Errorf("can't find env %s in project %s, error: %w", j.jobSpec.Env, j.workflow.Project, err)
			log.Error(err)
			return err
		}

		if env.IsSleeping() {
			return fmt.Errorf("env %s is sleeping", j.jobSpec.Env)
		}

		envInfo := &commonmodels.ZadigRestartEnvInformation{
			Env:        env.EnvName,
			EnvAlias:   env.Alias,
			Production: env.Production,
		}
		for _, serviceGroup := range env.Services {
			for _, service := range serviceGroup {
				envInfo.Services = append(envInfo.Services, service.ServiceName)
			}
		}
		envOptions = append(envOptions, envInfo)
	} else {
		envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: util.GetBoolPointer(j.jobSpec.Production),
		})
		if err != nil {
			log.Errorf("can't list envs in project %s, error: %w", j.workflow.Project, err)
			return err
		}

		for _, env := range envs {
			// skip the sleeping envs
			if env.IsSleeping() {
				continue
			}

			if !ticket.IsAllowedEnv(j.workflow.Project, env.EnvName) {
				continue
			}

			services := make([]string, 0)
			for _, serviceGroup := range env.Services {
				for _, service := range serviceGroup {
					services = append(services, service.ServiceName)
				}
			}

			envInfo := &commonmodels.ZadigRestartEnvInformation{
				Env:        env.EnvName,
				EnvAlias:   env.Alias,
				Production: env.Production,
				Services:   services,
			}

			envOptions = append(envOptions, envInfo)
		}
	}

	j.jobSpec.EnvOptions = envOptions
	return nil
}

func (j RestartJobController) ClearOptions() {
	j.jobSpec.EnvOptions = nil
}

func (j RestartJobController) ClearSelection() {
	j.jobSpec.Services = make([]string, 0)
}

func (j RestartJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.jobSpec.Env})
	if err != nil {
		return nil, fmt.Errorf("env %s not exists", j.jobSpec.Env)
	}

	// get deploy info from previous build job
	if j.jobSpec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}

		referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		restartOrder, err := j.getReferredJobOrder(referredJob)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", referredJob, err)
		}

		services := []string{}
		for _, svc := range restartOrder {
			if _, ok := env.GetServiceMap()[svc]; ok {
				services = append(services, svc)
			}
		}

		j.jobSpec.Services = services
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	deployType := setting.K8SDeployType
	if templateProduct.ProductFeature.GetDeployType() == setting.HelmDeployType {
		deployType = setting.HelmDeployType
	}

	for jobSubTaskID, svc := range j.jobSpec.Services {
		jobTaskSpec := &commonmodels.JobTaskRestartSpec{
			Env:                j.jobSpec.Env,
			SkipCheckRunStatus: j.jobSpec.SkipCheckRunStatus,
			ServiceName:        svc,
			DeployType:         deployType,
			ClusterID:          env.ClusterID,
			Production:         j.jobSpec.Production,
			Timeout:            timeout,
		}

		jobTask := &commonmodels.JobTask{
			Key:         genJobKey(j.name, svc),
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			DisplayName: genJobDisplayName(j.name, svc),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"service_name": svc,
			},
			JobType:       string(config.JobZadigRestart),
			Spec:          jobTaskSpec,
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		}

		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j RestartJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j RestartJobController) SetRepoCommitInfo() error {
	return nil
}

func (j RestartJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	resp = append(resp, &commonmodels.KeyVal{
		Key:          strings.Join([]string{"job", j.name, "envName"}, "."),
		Value:        j.jobSpec.Env,
		Type:         "string",
		IsCredential: false,
	})

	if getAggregatedVariables {
	}

	svcs := make([]string, 0)
	for _, service := range j.jobSpec.Services {
		svcs = append(svcs, service)
	}

	resp = append(resp, &commonmodels.KeyVal{
		Key:          strings.Join([]string{"job", j.name, "SERVICES"}, "."),
		Value:        strings.Join(svcs, ","),
		Type:         "string",
		IsCredential: false,
	})

	if getRuntimeVariables {
	}

	if getPlaceHolderVariables {
	}

	return resp, nil
}

func (j RestartJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j RestartJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j RestartJobController) IsServiceTypeJob() bool {
	return true
}

func (j RestartJobController) getReferredJobOrder(referredJob string) ([]string, error) {
	serviceSet := sets.Set[string]{}
	ret := make([]string, 0)

	updateRet := func(serviceName string) {
		if serviceSet.Has(serviceName) {
			return
		}
		serviceSet.Insert(serviceName)
		ret = append(ret, serviceName)
	}

	found := false

orderLoop:
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != referredJob {
				continue
			}

			if job.JobType == config.JobFreestyle {
				freestyleSpec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, freestyleSpec); err != nil {
					return nil, err
				}

				if freestyleSpec.FreestyleJobType != config.ServiceFreeStyleJobType {
					return nil, fmt.Errorf("freestyle job type %s not supported in restart job", freestyleSpec.FreestyleJobType)
				}

				for _, service := range freestyleSpec.Services {
					updateRet(service.ServiceName)
				}

				found = true
				break orderLoop
			}

			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return nil, err
				}

				for _, build := range buildSpec.ServiceAndBuilds {
					updateRet(build.ServiceName)
				}

				found = true
				break orderLoop
			}

			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return nil, err
				}

				for _, distribute := range distributeSpec.Targets {
					updateRet(distribute.ServiceName)
				}

				found = true
				break orderLoop
			}

			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, err
				}
				for _, service := range deploySpec.Services {
					updateRet(service.ServiceName)
				}
				found = true
				break orderLoop
			}

			if job.JobType == config.JobZadigTesting {
				testingSpec := &commonmodels.ZadigTestingJobSpec{}
				if err := commonmodels.IToi(job.Spec, testingSpec); err != nil {
					return nil, err
				}

				if testingSpec.TestType != config.ServiceTestType {
					return nil, fmt.Errorf("testing job type %s not supported in restart job", testingSpec.TestType)
				}

				for _, svc := range testingSpec.DefaultServices {
					updateRet(svc.ServiceName)
				}

				found = true
				break orderLoop
			}

			if job.JobType == config.JobZadigScanning {
				scanningSpec := &commonmodels.ZadigScanningJobSpec{}
				if err := commonmodels.IToi(job.Spec, scanningSpec); err != nil {
					return nil, err
				}

				if scanningSpec.ScanningType != config.ServiceScanningType {
					return nil, fmt.Errorf("scanning job type %s not supported in restart job", scanningSpec.ScanningType)
				}

				for _, svc := range scanningSpec.DefaultServices {
					updateRet(svc.ServiceName)
				}

				found = true
				break orderLoop
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("qutoed service referrece of job %s not found", referredJob)
	}

	return ret, nil
}
