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
	"net/url"
	"strings"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	steptypes "github.com/koderover/zadig/v2/pkg/types/step"
	pkgutil "github.com/koderover/zadig/v2/pkg/util"
	util2 "github.com/koderover/zadig/v2/pkg/util"
)

type FreestyleJobController struct {
	*BasicInfo

	jobSpec *commonmodels.FreestyleJobSpec
}

func CreateFreestyleJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.FreestyleJobSpec)
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

	return FreestyleJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j FreestyleJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j FreestyleJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j FreestyleJobController) Validate(isExecution bool) error {
	for _, kv := range j.jobSpec.Envs {
		if kv.Type == commonmodels.Script {
			kv.FunctionReference = util2.FindVariableKeyRef(kv.CallFunction)
		}
	}

	if j.jobSpec.FreestyleJobType == config.ServiceFreeStyleJobType {
		err := commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			return e.ErrLicenseInvalid.AddDesc("")
		}
	}

	err := checkOutputNames(j.jobSpec.AdvancedSetting.Outputs)
	if err != nil {
		return err
	}

	return nil
}

func (j FreestyleJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.FreestyleJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Runtime = currJobSpec.Runtime
	j.jobSpec.AdvancedSetting = currJobSpec.AdvancedSetting
	j.jobSpec.Script = currJobSpec.Script
	j.jobSpec.ScriptType = currJobSpec.ScriptType
	j.jobSpec.JobName = currJobSpec.JobName
	j.jobSpec.ObjectStorageUpload = currJobSpec.ObjectStorageUpload
	j.jobSpec.DefaultServices = currJobSpec.DefaultServices
	if useUserInput {
		j.jobSpec.Repos = applyRepos(currJobSpec.Repos, j.jobSpec.Repos)
		j.jobSpec.Envs = applyKeyVals(currJobSpec.Envs, j.jobSpec.Envs, false)
	} else {
		j.jobSpec.Repos = currJobSpec.Repos
		j.jobSpec.Envs = currJobSpec.Envs
	}

	if useUserInput {
		for _, svc := range j.jobSpec.Services {
			svc.KeyVals = applyKeyVals(j.jobSpec.Envs, svc.KeyVals, false)
			svc.Repos = applyRepos(j.jobSpec.Repos, svc.Repos)
		}
	}

	return nil
}

func (j FreestyleJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j FreestyleJobController) ClearOptions() {
	return
}

func (j FreestyleJobController) ClearSelection() {
	return
}

func (j FreestyleJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := make([]*commonmodels.JobTask, 0)

	registries, err := commonservice.ListRegistryNamespaces("", true, logger)
	if err != nil {
		return nil, err
	}

	if j.jobSpec.FreestyleJobType == config.ServiceFreeStyleJobType {
		// get info from previous job
		if j.jobSpec.ServiceSource == config.SourceFromJob {
			referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
			targets, err := j.getReferredJobTargets(referredJob, j.jobSpec.RefRepos)
			if err != nil {
				return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", referredJob, err)
			}
			// clear service list to prevent old data from remaining
			j.jobSpec.Services = targets
		}
	}

	if j.jobSpec.FreestyleJobType == config.ServiceFreeStyleJobType {
		for subTaskID, svc := range j.jobSpec.Services {
			svc.KeyVals = applyKeyVals(j.jobSpec.Envs, svc.KeyVals, false)
			log.Infof("svc repo info:")
			for _, repo := range svc.Repos {
				log.Infof("repo: %+v", repo)
			}
			task, err := j.generateSubTask(taskID, subTaskID, registries, svc)
			if err != nil {
				return nil, err
			}
			resp = append(resp, task)
		}
	} else {
		task, err := j.generateSubTask(taskID, 0, registries, nil)
		if err != nil {
			return nil, err
		}
		resp = append(resp, task)
	}

	return resp, nil
}

func (j FreestyleJobController) SetRepo(repo *types.Repository) error {
	// first set repo in the config/input
	j.jobSpec.Repos = applyRepos(j.jobSpec.Repos, []*types.Repository{repo})

	// then update for each service selected
	for _, svc := range j.jobSpec.Services {
		svc.Repos = applyRepos(svc.Repos, []*types.Repository{repo})
	}

	return nil
}

func (j FreestyleJobController) SetRepoCommitInfo() error {
	err := setRepoInfo(j.jobSpec.Repos)
	if err != nil {
		return err
	}

	for _, svc := range j.jobSpec.Services {
		err = setRepoInfo(svc.Repos)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j FreestyleJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	if getAggregatedVariables {
		// no aggregated variables
	}

	if j.jobSpec.FreestyleJobType == config.NormalFreeStyleJobType {
		for _, kv := range j.jobSpec.Envs {
			jobKey := strings.Join([]string{"job", j.name}, ".")
			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, kv.Key),
				Value:        kv.GetValue(),
				Type:         "string",
				IsCredential: false,
			})
		}
	}

	if getRuntimeVariables {
		if j.jobSpec.FreestyleJobType == config.NormalFreeStyleJobType {
			for _, output := range j.jobSpec.AdvancedSetting.Outputs {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{"job", j.name, "output", output.Name}, "."),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
			// Add status variable for normal freestyle job
			resp = append(resp, &commonmodels.KeyVal{
				Key:          strings.Join([]string{"job", j.name, "status"}, "."),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})
		} else {
			for _, output := range j.jobSpec.AdvancedSetting.Outputs {
				if getServiceSpecificVariables {
					// TODO: ADD LOGIC, This thing is not allowed for now
				}
				if getPlaceHolderVariables {
					jobKey := strings.Join([]string{j.name, "<SERVICE>", "<MODULE>"}, ".")
					resp = append(resp, &commonmodels.KeyVal{
						Key:          strings.Join([]string{"job", jobKey, "output", output.Name}, "."),
						Value:        "",
						Type:         "string",
						IsCredential: false,
					})
				}
			}
			// Add placeholder status variable for service freestyle type
			if getPlaceHolderVariables {
				jobKey := strings.Join([]string{j.name, "<SERVICE>", "<MODULE>"}, ".")
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{"job", jobKey, "status"}, "."),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
		}
	}

	if getPlaceHolderVariables {
		if j.jobSpec.FreestyleJobType == config.ServiceFreeStyleJobType {
			jobKey := strings.Join([]string{"job", j.name, "<SERVICE>", "<MODULE>"}, ".")

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_NAME"),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_MODULE"),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})

			for _, kv := range j.jobSpec.Envs {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, kv.Key),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
		}
	}

	if getServiceSpecificVariables {
		for _, svc := range j.jobSpec.Services {
			jobKey := strings.Join([]string{"job", j.name, svc.ServiceName, svc.ServiceModule}, ".")

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_NAME"),
				Value:        svc.ServiceName,
				Type:         "string",
				IsCredential: false,
			})

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_MODULE"),
				Value:        svc.ServiceModule,
				Type:         "string",
				IsCredential: false,
			})

			for _, kv := range svc.KeyVals {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, kv.Key),
					Value:        kv.GetValue(),
					Type:         "string",
					IsCredential: false,
				})
			}
		}
	}

	return resp, nil
}

func (j FreestyleJobController) GetUsedRepos() ([]*types.Repository, error) {
	return j.jobSpec.Repos, nil
}

func (j FreestyleJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	for _, kv := range j.jobSpec.Envs {
		if kv.Key == key {
			resp, err := renderScriptedVariableOptions(option.ServiceName, option.ServiceModule, kv.Script, kv.CallFunction, option.Values)
			if err != nil {
				err = fmt.Errorf("Failed to render kv for key: %s, error: %s", key, err)
				return nil, err
			}
			return resp, nil
		}
	}
	return nil, fmt.Errorf("key: %s not found in job: %s", key, j.name)
}

func (j FreestyleJobController) IsServiceTypeJob() bool {
	return j.jobSpec.FreestyleJobType == config.ServiceFreeStyleJobType
}

func (j FreestyleJobController) getReferredJobTargets(jobName string, refRepos bool) ([]*commonmodels.FreeStyleServiceInfo, error) {
	serviceTargets := make([]*commonmodels.FreeStyleServiceInfo, 0)
	originTargetMap := make(map[string]*commonmodels.FreeStyleServiceInfo)
	for _, target := range j.jobSpec.Services {
		originTargetMap[target.GetKey()] = target
	}

	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return nil, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   build.ServiceName,
							ServiceModule: build.ServiceModule,
						},
					}
					if originTarget, ok := originTargetMap[target.GetKey()]; ok {
						target.Repos = originTarget.Repos
						target.KeyVals = originTarget.KeyVals
					} else {
						return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
					}

					if refRepos {
						target.Repos = mergeRepos(target.Repos, build.Repos)
					}

					log.Infof("getReferredJobTargets: workflow %s service %s, module %s, repos %v", j.workflow.Name, target.ServiceName, target.ServiceModule, target.Repos)

					serviceTargets = append(serviceTargets, target)
				}
				return serviceTargets, nil
			}
			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return nil, err
				}
				for _, distribute := range distributeSpec.Targets {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   distribute.ServiceName,
							ServiceModule: distribute.ServiceModule,
						},
					}
					if originTarget, ok := originTargetMap[target.GetKey()]; ok {
						target.Repos = originTarget.Repos
						target.KeyVals = originTarget.KeyVals
					} else {
						return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
					}

					serviceTargets = append(serviceTargets, target)
				}
				return serviceTargets, nil
			}
			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, err
				}
				for _, svc := range deploySpec.Services {
					for _, module := range svc.Modules {
						target := &commonmodels.FreeStyleServiceInfo{
							ServiceWithModule: commonmodels.ServiceWithModule{
								ServiceName:   svc.ServiceName,
								ServiceModule: module.ServiceModule,
							},
						}
						if originTarget, ok := originTargetMap[target.GetKey()]; ok {
							target.Repos = originTarget.Repos
							target.KeyVals = originTarget.KeyVals
						} else {
							return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
						}

						serviceTargets = append(serviceTargets, target)
					}
				}
				return serviceTargets, nil
			}
			if job.JobType == config.JobZadigScanning {
				scanningSpec := &commonmodels.ZadigScanningJobSpec{}
				if err := commonmodels.IToi(job.Spec, scanningSpec); err != nil {
					return nil, err
				}
				for _, svc := range scanningSpec.ServiceAndScannings {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   svc.ServiceName,
							ServiceModule: svc.ServiceModule,
						},
					}
					if originTarget, ok := originTargetMap[target.GetKey()]; ok {
						target.Repos = originTarget.Repos
						target.KeyVals = originTarget.KeyVals
					} else {
						return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
					}

					if refRepos {
						target.Repos = mergeRepos(target.Repos, svc.Repos)
					}

					serviceTargets = append(serviceTargets, target)
				}
				return serviceTargets, nil
			}
			if job.JobType == config.JobZadigTesting {
				testingSpec := &commonmodels.ZadigTestingJobSpec{}
				if err := commonmodels.IToi(job.Spec, testingSpec); err != nil {
					return nil, err
				}
				for _, svc := range testingSpec.ServiceAndTests {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   svc.ServiceName,
							ServiceModule: svc.ServiceModule,
						},
					}
					if originTarget, ok := originTargetMap[target.GetKey()]; ok {
						target.Repos = originTarget.Repos
						target.KeyVals = originTarget.KeyVals
					} else {
						return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
					}

					if refRepos {
						target.Repos = mergeRepos(target.Repos, svc.Repos)
					}

					serviceTargets = append(serviceTargets, target)
				}
				return serviceTargets, nil
			}
			if job.JobType == config.JobFreestyle {
				deploySpec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, err
				}
				if deploySpec.FreestyleJobType != config.ServiceFreeStyleJobType {
					return nil, fmt.Errorf("freestyle job type %s not supported in reference", deploySpec.FreestyleJobType)
				}
				for _, svc := range deploySpec.Services {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   svc.ServiceName,
							ServiceModule: svc.ServiceModule,
						},
					}
					if originTarget, ok := originTargetMap[target.GetKey()]; ok {
						target.Repos = originTarget.Repos
						target.KeyVals = originTarget.KeyVals
					} else {
						return nil, fmt.Errorf("refered job %s target %s not found", jobName, target.GetKey())
					}

					if refRepos {
						target.Repos = mergeRepos(target.Repos, svc.Repos)
					}

					serviceTargets = append(serviceTargets, target)
				}
				return serviceTargets, nil
			}
		}
	}
	return nil, fmt.Errorf("FreeStyleJob: refered job %s not found", jobName)
}

func (j FreestyleJobController) generateSubTask(taskID int64, jobSubTaskID int, registries []*commonmodels.RegistryNamespace, service *commonmodels.FreeStyleServiceInfo) (*commonmodels.JobTask, error) {
	basicImage, err := commonrepo.NewBasicImageColl().Find(j.jobSpec.Runtime.ImageID)
	if err != nil {
		return nil, fmt.Errorf("failed to find base image: %s,error :%v", j.jobSpec.Runtime.ImageID, err)
	}

	taskRunProperties := &commonmodels.JobProperties{
		Timeout:             j.jobSpec.AdvancedSetting.Timeout,
		ResourceRequest:     j.jobSpec.AdvancedSetting.ResourceRequest,
		ResReqSpec:          j.jobSpec.AdvancedSetting.ResReqSpec,
		Infrastructure:      j.jobSpec.Runtime.Infrastructure,
		VMLabels:            j.jobSpec.Runtime.VMLabels,
		ClusterID:           j.jobSpec.AdvancedSetting.ClusterID,
		ClusterSource:       j.jobSpec.AdvancedSetting.ClusterSource,
		StrategyID:          j.jobSpec.AdvancedSetting.StrategyID,
		BuildOS:             basicImage.Value,
		ImageFrom:           j.jobSpec.Runtime.ImageFrom,
		ImageID:             j.jobSpec.Runtime.ImageID,
		Registries:          registries,
		ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, j.jobSpec.AdvancedSetting.ShareStorageInfo, j.workflow.Name, taskID),
		UseHostDockerDaemon: j.jobSpec.AdvancedSetting.UseHostDockerDaemon,
		ServiceName:         "",
		CustomAnnotations:   j.jobSpec.AdvancedSetting.CustomAnnotations,
		CustomLabels:        j.jobSpec.AdvancedSetting.CustomLabels,
	}

	if service != nil {
		taskRunProperties.CustomEnvs = service.KeyVals.ToKVList()
	} else {
		taskRunProperties.CustomEnvs = j.jobSpec.Envs.ToKVList()
	}

	paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
	envs := mergeKeyVals(taskRunProperties.CustomEnvs, paramEnvs)

	jobDisplayName := genJobDisplayName(j.name)
	jobKey := genJobKey(j.name)
	jobName := GenJobName(j.workflow, j.name, jobSubTaskID)
	jobInfo := map[string]string{
		JobNameKey: j.name,
	}
	if service != nil {
		jobDisplayName = genJobDisplayName(j.name, service.ServiceName, service.ServiceModule)
		jobKey = genJobKey(j.name, service.ServiceName, service.ServiceModule)
		jobInfo["service_name"] = service.ServiceName
		jobInfo["service_module"] = service.ServiceModule
		renderRepos(service.Repos, applyKeyVals(j.jobSpec.Envs, service.KeyVals, false).ToKVList())
	} else {
		renderRepos(j.jobSpec.Repos, envs)
	}

	envs = append(envs, getFreestyleJobVariables(taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, j.jobSpec.Runtime.Infrastructure, service, registries, j.jobSpec.Repos)...)
	taskRunProperties.Envs = envs

	for _, env := range taskRunProperties.Envs {
		if env.Type == commonmodels.MultiSelectType {
			env.Value = strings.Join(env.ChoiceValue, ",")
		}
	}

	if j.jobSpec.AdvancedSetting.Storages != nil && j.jobSpec.AdvancedSetting.Storages.Enabled {
		if len(j.jobSpec.AdvancedSetting.Storages.StoragesProperties) > 0 {
			newStorages := make([]*types.NFSProperties, 0)
			for _, storage := range j.jobSpec.AdvancedSetting.Storages.StoragesProperties {
				newStorage := &types.NFSProperties{}
				err = pkgutil.DeepCopy(newStorage, storage)
				if err != nil {
					return nil, fmt.Errorf("failed to deep copy storage: %v", err)
				}

				newStorage.MountPath = commonutil.RenderEnv(storage.MountPath, taskRunProperties.Envs)
				newStorage.Subpath = commonutil.RenderEnv(storage.Subpath, taskRunProperties.Envs)
				newStorages = append(newStorages, newStorage)
			}

			taskRunProperties.Storages = newStorages
		}
	}

	repos := j.jobSpec.Repos
	if service != nil {
		repos = service.Repos
	}

	steps, err := j.generateStepTask(jobName, repos)
	if err != nil {
		return nil, err
	}
	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{
		Properties: *taskRunProperties,
		Steps:      steps,
	}

	jobTask := &commonmodels.JobTask{
		Key:           jobKey,
		Name:          jobName,
		DisplayName:   jobDisplayName,
		OriginName:    j.name,
		JobInfo:       jobInfo,
		JobType:       string(config.JobFreestyle),
		Spec:          jobTaskSpec,
		Timeout:       j.jobSpec.AdvancedSetting.Timeout,
		Outputs:       j.jobSpec.AdvancedSetting.Outputs,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,

		Infrastructure: j.jobSpec.Runtime.Infrastructure,
		VMLabels:       j.jobSpec.Runtime.VMLabels,
	}

	serviceName := ""
	serviceModule := ""
	if service != nil {
		serviceName = service.ServiceName
		serviceModule = service.ServiceModule
	}

	renderedTask, err := replaceServiceAndModulesForTask(jobTask, serviceName, serviceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to render service variables, error: %v", err)
	}

	return renderedTask, nil
}

func (j FreestyleJobController) generateStepTask(jobName string, repos []*types.Repository) ([]*commonmodels.StepTask, error) {
	resp := make([]*commonmodels.StepTask, 0)

	tools := make([]*steptypes.Tool, 0)
	for _, install := range j.jobSpec.Runtime.Installs {
		tools = append(tools, &steptypes.Tool{
			Name:    install.Name,
			Version: install.Version,
		})
	}

	resp = append(resp, &commonmodels.StepTask{
		Name:     stepNameInstallDeps,
		JobName:  jobName,
		StepType: config.StepTools,
		Spec: steptypes.StepToolInstallSpec{
			Installs: tools,
		},
	})

	availableCodeHosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
	}

	repos = applyRepos(j.jobSpec.Repos, repos)
	renderredRepo, err := renderReferredRepo(repos, j.workflow.Params)
	if err != nil {
		return nil, err
	}

	gitRepos, p4Repos := splitReposByType(renderredRepo)

	resp = append(resp, &commonmodels.StepTask{
		Name:     stepNameGit,
		JobName:  jobName,
		StepType: config.StepGit,
		Spec: steptypes.StepGitSpec{
			CodeHosts: availableCodeHosts,
			Repos:     gitRepos,
		},
	})

	resp = append(resp, &commonmodels.StepTask{
		Name:     stepNamePerforce,
		JobName:  jobName,
		StepType: config.StepPerforce,
		Spec:     steptypes.StepP4Spec{Repos: p4Repos},
	})

	if j.jobSpec.ScriptType == types.ScriptTypeShell || j.jobSpec.ScriptType == "" {
		resp = append(resp, &commonmodels.StepTask{
			Name:     stepNameShell,
			JobName:  jobName,
			StepType: config.StepShell,
			Spec:     steptypes.StepShellSpec{Scripts: append(strings.Split(replaceWrapLine(j.jobSpec.Script), "\n"), outputScript(j.jobSpec.AdvancedSetting.Outputs, j.jobSpec.Runtime.Infrastructure)...)},
		})
	} else if j.jobSpec.ScriptType == types.ScriptTypeBatchFile {
		resp = append(resp, &commonmodels.StepTask{
			Name:     stepNameBatchFile,
			JobName:  jobName,
			StepType: config.StepBatchFile,
			Spec:     steptypes.StepShellSpec{Scripts: []string{j.jobSpec.Script}},
		})
	} else if j.jobSpec.ScriptType == types.ScriptTypePowerShell {
		resp = append(resp, &commonmodels.StepTask{
			Name:     stepNameShell,
			JobName:  jobName,
			StepType: config.StepBatchFile,
			Spec:     steptypes.StepShellSpec{Scripts: []string{j.jobSpec.Script}},
		})
	}

	resp = append(resp, &commonmodels.StepTask{
		Name:     "debug-after",
		StepType: config.StepDebugAfter,
	})

	if j.jobSpec.ObjectStorageUpload != nil && j.jobSpec.ObjectStorageUpload.Enabled {
		detailList := make([]*steptypes.Upload, 0)
		for _, detail := range j.jobSpec.ObjectStorageUpload.UploadDetail {
			detailList = append(detailList, &steptypes.Upload{
				FilePath:        detail.FilePath,
				AbsFilePath:     detail.AbsFilePath,
				DestinationPath: detail.DestinationPath,
			})
		}
		resp = append(resp, &commonmodels.StepTask{
			Name:     "debug-after",
			StepType: config.StepArchive,
			Spec: steptypes.StepArchiveSpec{
				UploadDetail:    detailList,
				ObjectStorageID: j.jobSpec.ObjectStorageUpload.ObjectStorageID,
			},
		})
	}

	return resp, nil
}

func getFreestyleJobVariables(taskID int64, project, workflowName, workflowDisplayName, infrastructure string, serviceAndImage *commonmodels.FreeStyleServiceInfo, registries []*commonmodels.RegistryNamespace, repos []*types.Repository) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, prepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)
	// repo envs
	ret = append(ret, getReposVariables(repos)...)
	// service envs
	if serviceAndImage != nil {
		ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: serviceAndImage.ServiceName, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: serviceAndImage.ServiceModule, IsCredential: false})
	}
	// registry envs
	for _, registry := range registries {
		ret = append(ret, &commonmodels.KeyVal{Key: registry.Namespace + "_REGISTRY_HOST", Value: registry.RegAddr, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: registry.Namespace + "_REGISTRY_AK", Value: registry.AccessKey, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: registry.Namespace + "_REGISTRY_SK", Value: registry.SecretKey, IsCredential: true})
	}

	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), project, workflowName, taskID, url.QueryEscape(workflowDisplayName))
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})

	// TODO: remove it
	ret = append(ret, &commonmodels.KeyVal{Key: "GIT_SSL_NO_VERIFY", Value: "true", IsCredential: false})
	return ret
}
