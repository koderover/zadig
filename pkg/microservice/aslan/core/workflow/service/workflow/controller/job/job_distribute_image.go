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
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

const (
	DistributeTimeout int64 = 10

	// WorkflowInputImageTagVariable PreBuildImageTagVariable
	// These variables are not really workflow variables, will convert to real value or workflow variables
	// Not return from GetWorkflowGlobalVars function, instead of frontend
	WorkflowInputImageTagVariable = "{{.workflow.input.imageTag}}"
	PreBuildImageTagVariable      = "{{.job.preBuild.imageTag}}"
	PreJobImageTagVariable        = "{{.job.preJob.imageTag}}"
)

type DistributeImageJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigDistributeImageJobSpec
}

func CreateDistributeImageJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigDistributeImageJobSpec)
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

	return DistributeImageJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j DistributeImageJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j DistributeImageJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j DistributeImageJobController) Validate(isExecution bool) error {
	if j.jobSpec.Source != config.SourceFromJob {
		return nil
	}
	jobRankMap := GetJobRankMap(j.workflow.Stages)
	buildJobRank, ok := jobRankMap[j.jobSpec.JobName]
	if !ok || buildJobRank >= jobRankMap[j.name] {
		return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.JobName, j.name)
	}

	return nil
}

func (j DistributeImageJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	latestSpec := new(commonmodels.ZadigDistributeImageJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	// source is a bit tricky: if the saved args has a source of fromjob, but it has been change to runtime in the config
	// we need to not only update its source but also set services to empty slice.
	if useUserInput {
		if j.jobSpec.Source == config.SourceFromJob && latestSpec.Source == config.SourceRuntime {
			j.jobSpec.Targets = make([]*commonmodels.DistributeTarget, 0)
		}
	} else {
		j.jobSpec.Targets = latestSpec.Targets
	}
	j.jobSpec.Source = latestSpec.Source

	if j.jobSpec.Source == config.SourceFromJob {
		j.jobSpec.JobName = latestSpec.JobName
	} else {
		j.jobSpec.SourceRegistryID = latestSpec.SourceRegistryID
	}

	j.jobSpec.TargetRegistryID = latestSpec.TargetRegistryID
	j.jobSpec.Timeout = latestSpec.Timeout
	j.jobSpec.ClusterID = latestSpec.ClusterID
	j.jobSpec.StrategyID = latestSpec.StrategyID
	j.jobSpec.EnableTargetImageTagRule = latestSpec.EnableTargetImageTagRule
	j.jobSpec.TargetImageTagRule = latestSpec.TargetImageTagRule
	j.jobSpec.Architecture = latestSpec.Architecture

	return nil
}

func (j DistributeImageJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return fmt.Errorf("get services map error: %v", err)
	}

	options := make([]*commonmodels.DistributeTarget, 0)
	for _, svc := range servicesMap {
		for _, module := range svc.Containers {
			if ticket.IsAllowedService(j.workflow.Project, svc.ServiceName, module.Name) {
				options = append(options, &commonmodels.DistributeTarget{
					ServiceName:   svc.ServiceName,
					ServiceModule: module.Name,
					ImageName:     util.ExtractImageName(module.Image),
				})
			}
		}
	}

	j.jobSpec.TargetOptions = options
	return nil
}

func (j DistributeImageJobController) ClearOptions() {
	j.jobSpec.TargetOptions = make([]*commonmodels.DistributeTarget, 0)
	return
}

func (j DistributeImageJobController) ClearSelection() {
	j.jobSpec.Targets = make([]*commonmodels.DistributeTarget, 0)
	return
}

func (j DistributeImageJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := make([]*commonmodels.JobTask, 0)

	var sourceReg *commonmodels.RegistryNamespace
	var targetReg *commonmodels.RegistryNamespace
	var err error

	switch j.jobSpec.Source {
	case config.SourceFromJob:
		serviceReferredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		targets, registryID, err := j.getReferredJobTargets(serviceReferredJob, j.jobSpec.JobName)
		if err != nil {
			return nil, fmt.Errorf("failed to get referred job info for distribute job: %s, error: %s", j.name, err)
		}

		j.jobSpec.SourceRegistryID = registryID

		sourceReg, err = commonservice.FindRegistryById(j.jobSpec.SourceRegistryID, true, logger)
		if err != nil {
			return resp, fmt.Errorf("source image registry: %s not found: %v", j.jobSpec.SourceRegistryID, err)
		}

		targetTagMap := map[string]commonmodels.DistributeTarget{}
		for _, target := range j.jobSpec.Targets {
			targetTagMap[getServiceKey(target.ServiceName, target.ServiceModule)] = *target
		}

		for _, target := range targets {
			if j.jobSpec.EnableTargetImageTagRule {
				target.TargetTag = strings.ReplaceAll(j.jobSpec.TargetImageTagRule, PreBuildImageTagVariable,
					fmt.Sprintf("{{.job.%s.%s.%s.output.%s}}", j.jobSpec.JobName, target.ServiceName, target.ServiceModule, IMAGETAGKEY))
				target.TargetTag = strings.ReplaceAll(j.jobSpec.TargetImageTagRule, PreJobImageTagVariable,
					fmt.Sprintf("{{.job.%s.%s.%s.output.%s}}", j.jobSpec.JobName, target.ServiceName, target.ServiceModule, IMAGETAGKEY))
				target.UpdateTag = true
			} else {
				target.TargetTag = targetTagMap[getServiceKey(target.ServiceName, target.ServiceModule)].TargetTag
				target.UpdateTag = targetTagMap[getServiceKey(target.ServiceName, target.ServiceModule)].UpdateTag
			}
		}

		j.jobSpec.Targets = targets
	case config.SourceRuntime:
		sourceReg, err = commonservice.FindRegistryById(j.jobSpec.SourceRegistryID, true, logger)
		if err != nil {
			return resp, fmt.Errorf("source image registry: %s not found: %v", j.jobSpec.SourceRegistryID, err)
		}

		for _, target := range j.jobSpec.Targets {
			if target.ImageName == "" {
				target.SourceImage = getImage(target.ServiceModule, target.SourceTag, sourceReg)
			} else {
				target.SourceImage = getImage(target.ImageName, target.SourceTag, sourceReg)
			}
			if j.jobSpec.EnableTargetImageTagRule {
				target.TargetTag = strings.ReplaceAll(j.jobSpec.TargetImageTagRule,
					WorkflowInputImageTagVariable, target.SourceTag)
			}
			target.UpdateTag = true
		}
	}

	targetReg, err = commonservice.FindRegistryById(j.jobSpec.TargetRegistryID, true, logger)
	if err != nil {
		return resp, fmt.Errorf("target image registry: %s not found: %v", j.jobSpec.TargetRegistryID, err)
	}

	stepSpec := &step.StepImageDistributeSpec{
		Type:           j.jobSpec.DistributeMethod,
		SourceRegistry: getRegistry(sourceReg),
		TargetRegistry: getRegistry(targetReg),
		Architecture:   j.jobSpec.Architecture,
	}
	for _, target := range j.jobSpec.Targets {
		// for other job refer current latest image.
		targetKey := strings.Join([]string{j.name, target.ServiceName, target.ServiceModule}, ".")
		target.TargetImage = job.GetJobOutputKey(targetKey, "IMAGE")

		targetTag := target.TargetTag
		targetTag = strings.ReplaceAll(targetTag, "<SERVICE>", target.ServiceName)
		targetTag = strings.ReplaceAll(targetTag, "<MODULE>", target.ServiceModule)

		stepSpec.DistributeTarget = append(stepSpec.DistributeTarget, &step.DistributeTaskTarget{
			SourceImage:   target.SourceImage,
			ServiceName:   target.ServiceName,
			ServiceModule: target.ServiceModule,
			TargetTag:     targetTag,
			UpdateTag:     target.UpdateTag,
		})
	}

	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{
		Properties: commonmodels.JobProperties{
			Timeout:           j.jobSpec.Timeout,
			ResourceRequest:   setting.MinRequest,
			ClusterID:         j.jobSpec.ClusterID,
			StrategyID:        j.jobSpec.StrategyID,
			BuildOS:           "focal",
			ImageFrom:         commonmodels.ImageFromKoderover,
			CustomAnnotations: j.jobSpec.CustomAnnotations,
			CustomLabels:      j.jobSpec.CustomLabels,
		},
		Steps: []*commonmodels.StepTask{
			{
				Name:     "distribute",
				StepType: config.StepDistributeImage,
				Spec:     stepSpec,
			},
		},
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:       string(config.JobZadigDistributeImage),
		Spec:          jobTaskSpec,
		Timeout:       getTimeout(j.jobSpec.Timeout),
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j DistributeImageJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j DistributeImageJobController) SetRepoCommitInfo() error {
	return nil
}

func (j DistributeImageJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	if getAggregatedVariables {
		// no aggregated variable list
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "IMAGES"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}

	if getRuntimeVariables {
		if getPlaceHolderVariables {
			resp = append(resp, &commonmodels.KeyVal{
				Key:          strings.Join([]string{"job", j.name, "<SERVICE>", "<MODULE>", "output", "IMAGE"}, "."),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})
		}
	}

	if getPlaceHolderVariables {
		// no normal placeholder variables
	}

	if getServiceSpecificVariables {
		// no normal Service Specific variables
	}

	return resp, nil
}

func (j DistributeImageJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j DistributeImageJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j DistributeImageJobController) IsServiceTypeJob() bool {
	return true
}

func (j DistributeImageJobController) getReferredJobTargets(serviceReferredJob, imageReferredJob string) ([]*commonmodels.DistributeTarget, string, error) {
	serviceTargets := make([]*commonmodels.DistributeTarget, 0)
	var sourceRegistryID string
	found := false
serviceLoop:
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != serviceReferredJob {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return nil, "", fmt.Errorf("failed to decode build job spec, error: %s", err)
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					serviceTargets = append(serviceTargets, &commonmodels.DistributeTarget{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
					})
				}
				sourceRegistryID = buildSpec.DockerRegistryID
				found = true
				break serviceLoop
			}

			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return nil, "", fmt.Errorf("failed to decode distribute job spec, error: %s", err)
				}
				for _, distribute := range distributeSpec.Targets {
					serviceTargets = append(serviceTargets, &commonmodels.DistributeTarget{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
					})
				}
				sourceRegistryID = distributeSpec.TargetRegistryID
				found = true
				break serviceLoop
			}

			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, "", fmt.Errorf("failed to decode deploy job spec, error: %s", err)
				}
				for _, svc := range deploySpec.Services {
					for _, module := range svc.Modules {
						serviceTargets = append(serviceTargets, &commonmodels.DistributeTarget{
							ServiceName:   svc.ServiceName,
							ServiceModule: module.ServiceModule,
						})
					}
				}

				envInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
					EnvName: deploySpec.Env,
					Name:    j.workflow.Project,
				})

				if err != nil {
					return nil, "", fmt.Errorf("failed to get deploy job %s env's registry info, error: %s", deploySpec.Env, err)
				}

				sourceRegistryID = envInfo.RegistryID
				found = true
				break serviceLoop
			}
		}
	}

	if !found {
		return nil, "", fmt.Errorf("ImageDistributeJob: referred job %s not found", serviceReferredJob)
	}

	// then we determine the image for the selected job, use the output for each module is enough
	for _, svc := range serviceTargets {
		// generate real job keys
		key := job.GetJobOutputKey(fmt.Sprintf("%s.%s.%s", imageReferredJob, svc.ServiceName, svc.ServiceModule), IMAGEKEY)

		svc.SourceImage = key
	}

	return serviceTargets, sourceRegistryID, nil
}

func getServiceKey(serviceName, serviceModule string) string {
	return fmt.Sprintf("%s/%s", serviceName, serviceModule)
}

func getImage(name, tag string, reg *commonmodels.RegistryNamespace) string {
	image := fmt.Sprintf("%s/%s:%s", reg.RegAddr, name, tag)
	if len(reg.Namespace) > 0 {
		image = fmt.Sprintf("%s/%s/%s:%s", reg.RegAddr, reg.Namespace, name, tag)
	}
	image = strings.TrimPrefix(image, "http://")
	image = strings.TrimPrefix(image, "https://")
	return image
}

func getRegistry(regDetail *commonmodels.RegistryNamespace) *step.RegistryNamespace {
	reg := &step.RegistryNamespace{
		RegAddr:   regDetail.RegAddr,
		Namespace: regDetail.Namespace,
		AccessKey: regDetail.AccessKey,
		SecretKey: regDetail.SecretKey,
	}
	if regDetail.AdvancedSetting != nil {
		reg.TLSEnabled = regDetail.AdvancedSetting.TLSEnabled
		reg.TLSCert = regDetail.AdvancedSetting.TLSCert
	}
	return reg
}

func getTimeout(timeout int64) int64 {
	if timeout == 0 {
		return DistributeTimeout
	}
	return timeout
}
