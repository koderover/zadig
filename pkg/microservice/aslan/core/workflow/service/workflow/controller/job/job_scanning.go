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

package job

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

// TODO: scanning => scanning_options in configuration
// TODO: service_and_scannings => service_scanning_options in configuration

type ScanningJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigScanningJobSpec
}

func CreateScanningJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigScanningJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return ScanningJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j ScanningJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j ScanningJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j ScanningJobController) Validate(isExecution bool) error {
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

func (j ScanningJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigScanningJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ScanningType = currJobSpec.ScanningType
	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.JobName = currJobSpec.JobName
	j.jobSpec.OriginJobName = currJobSpec.OriginJobName
	j.jobSpec.RefRepos = currJobSpec.RefRepos
	j.jobSpec.ScanningOptions = currJobSpec.ScanningOptions
	j.jobSpec.ServiceScanningOptions = currJobSpec.ServiceScanningOptions

	scanSvc := commonservice.NewScanningService()

	// merge the input with the configuration
	switch j.jobSpec.ScanningType {
	case config.ServiceScanningType:
		configuredServiceScanningMap := make(map[string]*commonmodels.ScanningModule)
		for _, configuredSvcScanning := range j.jobSpec.ServiceScanningOptions {
			key := fmt.Sprintf("%s++%s", configuredSvcScanning.ServiceName, configuredSvcScanning.ServiceModule)
			scanInfo, err := scanSvc.GetByName(j.workflow.Project, configuredSvcScanning.Name)
			if err != nil {
				return err
			}
			configuredSvcScanning.ScanningModule.KeyVals = applyKeyVals(scanInfo.Envs.ToRuntimeList(), configuredSvcScanning.KeyVals, true)
			configuredSvcScanning.ScanningModule.Repos = applyRepos(scanInfo.Repos, configuredSvcScanning.Repos)
			configuredServiceScanningMap[key] = configuredSvcScanning.ScanningModule
		}

		newSelectedService := make([]*commonmodels.ServiceAndScannings, 0)
		for _, svc := range j.jobSpec.ServiceAndScannings {
			key := fmt.Sprintf("%s++%s", svc.ServiceName, svc.ServiceModule)
			if _, ok := configuredServiceScanningMap[key]; !ok {
				continue
			}
			svc.KeyVals = applyKeyVals(configuredServiceScanningMap[key].KeyVals, svc.KeyVals, false)
			svc.Repos = applyRepos(configuredServiceScanningMap[key].Repos, svc.Repos)
			newSelectedService = append(newSelectedService, svc)
		}
		j.jobSpec.ServiceAndScannings = newSelectedService
	default:
		for _, configuredScanning := range j.jobSpec.ScanningOptions {
			scanInfo, err := scanSvc.GetByName(j.workflow.Project, configuredScanning.Name)
			if err != nil {
				return err
			}
			configuredScanning.KeyVals = applyKeyVals(scanInfo.Envs.ToRuntimeList(), configuredScanning.KeyVals, true)
			configuredScanning.Repos = applyRepos(scanInfo.Repos, configuredScanning.Repos)
		}

		userInputMap := make(map[string]*commonmodels.ScanningModule)
		newSelectedScanning := make([]*commonmodels.ScanningModule, 0)
		for _, scanning := range j.jobSpec.Scannings {
			userInputMap[scanning.Name] = scanning
		}
		
		for _, option := range j.jobSpec.ScanningOptions {
			item := &commonmodels.ScanningModule{
				Name: option.Name,
				ProjectName: option.ProjectName,
				ShareStorageInfo: option.ShareStorageInfo,
				KeyVals: option.KeyVals,
				Repos: option.Repos,
			}
			if input, ok := userInputMap[option.Name]; ok {
				item.KeyVals = applyKeyVals(item.KeyVals, input.KeyVals, false)
				item.Repos = applyRepos(item.Repos, input.Repos)
			}
			newSelectedScanning = append(newSelectedScanning, item)
		}
		j.jobSpec.Scannings = newSelectedScanning
	}

	return nil
}

func (j ScanningJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	newScanningRange := make([]*commonmodels.ServiceAndScannings, 0)
	for _, svcScanning := range j.jobSpec.ServiceAndScannings {
		if ticket.IsAllowedService(j.workflow.Project, svcScanning.ServiceName, svcScanning.ServiceModule) {
			newScanningRange = append(newScanningRange, svcScanning)
		}
	}

	j.jobSpec.ServiceAndScannings = newScanningRange
	return nil
}

func (j ScanningJobController) ClearOptions() {
	return
}

func (j ScanningJobController) ClearSelection() {
	j.jobSpec.Scannings = make([]*commonmodels.ScanningModule, 0)
	j.jobSpec.ServiceAndScannings = make([]*commonmodels.ServiceAndScannings, 0)
}

func (j ScanningJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := make([]*commonmodels.JobTask, 0)

	if j.jobSpec.ScanningType == config.NormalScanningType {
		for subJobTaskID, scanning := range j.jobSpec.Scannings {
			jobTask, err := j.toJobTask(subJobTaskID, scanning, taskID, string(j.jobSpec.ScanningType), "", "", logger)
			if err != nil {
				return nil, err
			}
			resp = append(resp, jobTask)
		}
	}

	// get deploy info from previous build job
	if j.jobSpec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}
	}

	if j.jobSpec.ScanningType == config.ServiceScanningType {
		jobSubTaskID := 0
		targetsMap := make(map[string]*commonmodels.ServiceTestTarget)
		if j.jobSpec.Source == config.SourceFromJob {
			referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
			targets, err := j.getReferredJobTargets(referredJob)
			if err != nil {
				return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", referredJob, err)
			}
			for _, target := range targets {
				key := fmt.Sprintf("%s++%s", target.ServiceName, target.ServiceModule)
				targetsMap[key] = target
			}
		}
		for _, scanning := range j.jobSpec.ServiceAndScannings {
			if j.jobSpec.Source == config.SourceFromJob {
				key := fmt.Sprintf("%s++%s", scanning.ServiceName, scanning.ServiceModule)
				if _, ok := targetsMap[key]; !ok {
					// if a service is not referred but passed in, ignore it
					continue
				}
			}
			jobTask, err := j.toJobTask(jobSubTaskID, scanning.ScanningModule, taskID, string(j.jobSpec.ScanningType), scanning.ServiceName, scanning.ServiceModule, logger)
			if err != nil {
				return resp, err
			}
			jobSubTaskID++
			resp = append(resp, jobTask)
		}
	}

	return resp, nil
}

func (j ScanningJobController) SetRepo(repo *types.Repository) error {
	for _, scanning := range j.jobSpec.Scannings {
		scanning.Repos = applyRepos(scanning.Repos, []*types.Repository{repo})
	}
	for _, serviceAndScaning := range j.jobSpec.ServiceAndScannings {
		serviceAndScaning.Repos = applyRepos(serviceAndScaning.Repos, []*types.Repository{repo})
	}
	return nil
}

func (j ScanningJobController) SetRepoCommitInfo() error {
	for _, scanning := range j.jobSpec.Scannings {
		if err := setRepoInfo(scanning.Repos); err != nil {
			return err
		}
	}
	for _, serviceAndScaning := range j.jobSpec.ServiceAndScannings {
		if err := setRepoInfo(serviceAndScaning.Repos); err != nil {
			return err
		}
	}
	return nil
}

func (j ScanningJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	if getAggregatedVariables {
		// No aggregated variables
	}

	if j.jobSpec.ScanningType == config.NormalScanningType || j.jobSpec.ScanningType == "" {
		targets := j.jobSpec.ScanningOptions
		if useUserInputValue {
			targets = j.jobSpec.Scannings
		}
		for _, scan := range targets {
			jobKey := strings.Join([]string{j.name, scan.Name}, ".")
			for _, kv := range scan.KeyVals {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{"job", jobKey, kv.Key}, "."),
					Value:        kv.GetValue(),
					Type:         "string",
					IsCredential: false,
				})
			}
		}
	}

	if getRuntimeVariables {
		scanningNames := []string{}
		if j.jobSpec.ScanningType == config.NormalScanningType {
			for _, scanning := range j.jobSpec.ScanningOptions {
				scanningNames = append(scanningNames, scanning.Name)
			}
		} else if j.jobSpec.ScanningType == config.ServiceScanningType {
			for _, scanning := range j.jobSpec.ServiceScanningOptions {
				scanningNames = append(scanningNames, scanning.Name)
			}
		}
		scanningInfos, _, err := commonrepo.NewScanningColl().List(&commonrepo.ScanningListOption{ScanningNames: scanningNames, ProjectName: j.workflow.Project}, 0, 0)
		if err != nil {
			log.Errorf("list scanning info failed: %v", err)
			return nil, err
		}
		for _, scanningInfo := range scanningInfos {
			if j.jobSpec.ScanningType == config.ServiceScanningType {
				if getPlaceHolderVariables {
					jobKey := strings.Join([]string{j.name, "<SERVICE>", "<MODULE>"}, ".")
					for _, output := range scanningInfo.Outputs {
						resp = append(resp,  &commonmodels.KeyVal{
							Key:          strings.Join([]string{"job", jobKey, "output", output.Name}, "."),
							Value:        "",
							Type:         "string",
							IsCredential: false,
						})
					}
				}
				if getServiceSpecificVariables {
					for _, scanning := range j.jobSpec.ServiceScanningOptions {
						if scanningInfo.Name != scanning.Name {
							continue
						}
						jobKey := strings.Join([]string{j.name, scanning.ServiceName, scanning.ServiceModule}, ".")
						for _, output := range scanningInfo.Outputs {
							resp = append(resp,  &commonmodels.KeyVal{
								Key:          strings.Join([]string{"job", jobKey, "output", output.Name}, "."),
								Value:        "",
								Type:         "string",
								IsCredential: false,
							})
						}
					}
				}
			} else {
				jobKey := strings.Join([]string{j.name, scanningInfo.Name}, ".")
				for _, output := range scanningInfo.Outputs {
					resp = append(resp, &commonmodels.KeyVal{
						Key:          strings.Join([]string{"job", jobKey, "output", output.Name}, "."),
						Value:        "",
						Type:         "string",
						IsCredential: false,
					})
				}
			}
		}
	}

	if getPlaceHolderVariables {
		jobKey := strings.Join([]string{"job", j.name, "<SERVICE>", "<MODULE>"}, ".")
		if j.jobSpec.ScanningType == config.ServiceScanningType {
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

			keySet := sets.NewString()
			for _, service := range j.jobSpec.ServiceScanningOptions {
				for _, keyVal := range service.KeyVals {
					keySet.Insert(keyVal.Key)
				}
			}

			for _, key := range keySet.List() {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{jobKey, key}, "."),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
		}
	}

	if getServiceSpecificVariables {
		targets := j.jobSpec.ServiceScanningOptions
		if useUserInputValue {
			targets = j.jobSpec.ServiceAndScannings
		}
		for _, service := range targets {
			jobKey := strings.Join([]string{"job", j.name, service.ServiceName, service.ServiceModule}, ".")
			for _, keyVal := range service.KeyVals {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, keyVal.Key),
					Value:        keyVal.GetValue(),
					Type:         "string",
					IsCredential: false,
				})
			}

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_NAME"),
				Value:        service.ServiceName,
				Type:         "string",
				IsCredential: false,
			})

			resp = append(resp, &commonmodels.KeyVal{
				Key:          fmt.Sprintf("%s.%s", jobKey, "SERVICE_MODULE"),
				Value:        service.ServiceModule,
				Type:         "string",
				IsCredential: false,
			})
		}
	}

	
	return resp, nil
}

func (j ScanningJobController) GetUsedRepos() ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	if j.jobSpec.ScanningType == config.NormalScanningType || j.jobSpec.ScanningType == "" {
		for _, scanning := range j.jobSpec.ScanningOptions {
			scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
			if err != nil {
				log.Errorf("find scanning: %s error: %v", scanning.Name, err)
				continue
			}
			resp = append(resp, applyRepos(scanningInfo.Repos, scanning.Repos)...)
		}
	} else if j.jobSpec.ScanningType == config.ServiceScanningType {
		for _, scanning := range j.jobSpec.ServiceScanningOptions {
			scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
			if err != nil {
				log.Errorf("find scanning: %s error: %v", scanning.Name, err)
				continue
			}
			resp = append(resp, applyRepos(scanningInfo.Repos, scanning.Repos)...)
		}
	}
	
	return resp, nil
}

func (j ScanningJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j ScanningJobController) IsServiceTypeJob() bool {
	return j.jobSpec.ScanningType == config.ServiceScanningType
}

func (j ScanningJobController) toJobTask(jobSubTaskID int, scanning *commonmodels.ScanningModule, taskID int64, scanningType, serviceName, serviceModule string, logger *zap.SugaredLogger) (*commonmodels.JobTask, error) {
	scanningInfo, err := commonrepo.NewScanningColl().Find(j.workflow.Project, scanning.Name)
	if err != nil {
		return nil, fmt.Errorf("find scanning: %s error: %v", scanning.Name, err)
	}

	if err := fillScanningDetail(scanningInfo); err != nil {
		return nil, err
	}

	basicImage, err := commonrepo.NewBasicImageColl().Find(scanningInfo.ImageID)
	if err != nil {
		return nil, fmt.Errorf("find basic image: %s error: %v", scanningInfo.ImageID, err)
	}
	registries, err := commonservice.ListRegistryNamespaces("", true, logger)
	if err != nil {
		return nil, fmt.Errorf("list registries error: %v", err)
	}
	var timeout int64
	if scanningInfo.AdvancedSetting != nil {
		timeout = scanningInfo.AdvancedSetting.Timeout
	}
	jobTaskSpec := new(commonmodels.JobTaskFreestyleSpec)

	jobInfo := map[string]string{
		JobNameKey:      j.name,
		"scanning_name": scanning.Name,
		"scanning_type": scanningType,
	}

	if scanningType == string(config.ServiceScanningType) {
		jobInfo["service_name"] = serviceName
		jobInfo["service_module"] = serviceModule
	}

	jobName := GenJobName(j.workflow, j.name, jobSubTaskID)
	jobKey := genJobKey(j.name, scanning.Name)
	jobDisplayName := genJobDisplayName(j.name, scanning.Name)
	if scanningType == string(config.ServiceScanningType) {
		jobKey = genJobKey(j.name, serviceName, serviceModule)
		jobDisplayName = genJobDisplayName(j.name, serviceName, serviceModule)
	}

	jobTask := &commonmodels.JobTask{
		Key:            jobKey,
		Name:           jobName,
		DisplayName:    jobDisplayName,
		OriginName:     j.name,
		JobInfo:        jobInfo,
		JobType:        string(config.JobZadigScanning),
		Spec:           jobTaskSpec,
		Timeout:        timeout,
		Outputs:        ensureScanningOutputs(scanningInfo.Outputs),
		Infrastructure: scanningInfo.Infrastructure,
		VMLabels:       scanningInfo.VMLabels,
		ErrorPolicy:    j.errorPolicy,
	}

	paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
	envs := mergeKeyVals(applyKeyVals(scanningInfo.Envs.ToRuntimeList(), scanning.KeyVals, true).ToKVList(), paramEnvs)

	scanningImage := basicImage.Value
	if scanningImage == "sonarsource/sonar-scanner-cli" {
		scanningImage += ":5.0.1"
	}
	if basicImage.ImageFrom == commonmodels.ImageFromKoderover {
		scanningImage = strings.ReplaceAll(config.ReaperImage(), "${BuildOS}", basicImage.Value)
	}
	jobTaskSpec.Properties = commonmodels.JobProperties{
		Timeout:             timeout,
		ResourceRequest:     scanningInfo.AdvancedSetting.ResReq,
		ResReqSpec:          scanningInfo.AdvancedSetting.ResReqSpec,
		ClusterID:           scanningInfo.AdvancedSetting.ClusterID,
		StrategyID:          scanningInfo.AdvancedSetting.StrategyID,
		BuildOS:             scanningImage,
		ImageFrom:           setting.ImageFromCustom,
		Envs:                append(envs, getScanningJobVariables(scanning.Repos, taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, jobTask.Infrastructure, scanningType, serviceName, serviceModule, scanning.Name)...),
		Registries:          registries,
		ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, scanning.ShareStorageInfo, j.workflow.Name, taskID),
		CustomAnnotations:   scanningInfo.AdvancedSetting.CustomAnnotations,
		CustomLabels:        scanningInfo.AdvancedSetting.CustomLabels,
	}

	if scanningType == string(config.ServiceScanningType) {
		renderedEnv, err := replaceServiceAndModules(jobTaskSpec.Properties.Envs, serviceName, serviceModule)
		if err != nil {
			return nil, fmt.Errorf("failed to render service variables, error: %v", err)
		}

		jobTaskSpec.Properties.Envs = renderedEnv
	}

	cacheS3 := &commonmodels.S3Storage{}
	clusterInfo, err := commonrepo.NewK8SClusterColl().Get(scanningInfo.AdvancedSetting.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster: %s, error: %v", scanningInfo.AdvancedSetting.ClusterID, err)
	}

	if jobTask.Infrastructure == setting.JobVMInfrastructure {
		jobTaskSpec.Properties.CacheEnable = scanningInfo.AdvancedSetting.Cache.CacheEnable
		jobTaskSpec.Properties.CacheDirType = scanningInfo.AdvancedSetting.Cache.CacheDirType
		jobTaskSpec.Properties.CacheUserDir = scanningInfo.AdvancedSetting.Cache.CacheUserDir
	} else {
		if clusterInfo.Cache.MediumType == "" {
			jobTaskSpec.Properties.CacheEnable = false
		} else {
			jobTaskSpec.Properties.Cache = clusterInfo.Cache
			if scanningInfo.AdvancedSetting.Cache != nil {
				jobTaskSpec.Properties.CacheEnable = scanningInfo.AdvancedSetting.Cache.CacheEnable
				jobTaskSpec.Properties.CacheDirType = scanningInfo.AdvancedSetting.Cache.CacheDirType
				jobTaskSpec.Properties.CacheUserDir = scanningInfo.AdvancedSetting.Cache.CacheUserDir

				if jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
					cacheS3, err = commonrepo.NewS3StorageColl().Find(jobTaskSpec.Properties.Cache.ObjectProperties.ID)
					if err != nil {
						return nil, fmt.Errorf("find cache s3 storage: %s error: %v", jobTaskSpec.Properties.Cache.ObjectProperties.ID, err)
					}
				}
			} else {
				jobTaskSpec.Properties.CacheEnable = false
			}
		}
	}

	if len(scanning.Repos) > 0 {
		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
			Key:   "BRANCH",
			Value: scanning.Repos[0].Branch,
		})
	}

	// init tools install step
	tools := []*step.Tool{}
	for _, tool := range scanningInfo.Installs {
		tools = append(tools, &step.Tool{
			Name:    tool.Name,
			Version: tool.Version,
		})
	}
	toolInstallStep := &commonmodels.StepTask{
		Name:     fmt.Sprintf("%s-%s", scanning.Name, "tool-install"),
		JobName:  jobTask.Name,
		StepType: config.StepTools,
		Spec:     step.StepToolInstallSpec{Installs: tools},
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, toolInstallStep)

	// init download object cache step
	if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
		cacheDir := "/workspace"
		if jobTaskSpec.Properties.CacheDirType == types.UserDefinedCacheDir {
			cacheDir = jobTaskSpec.Properties.CacheUserDir
		}
		downloadArchiveStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", scanning.Name, "download-archive"),
			JobName:  jobTask.Name,
			StepType: config.StepDownloadArchive,
			Spec: step.StepDownloadArchiveSpec{
				UnTar:      true,
				IgnoreErr:  true,
				FileName:   setting.ScanningOSSCacheFileName,
				ObjectPath: getScanningJobCacheObjectPath(j.workflow.Name, scanning.Name),
				DestDir:    cacheDir,
				S3:         modelS3toS3(cacheS3),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArchiveStep)
	}

	repos := applyRepos(scanningInfo.Repos, scanning.Repos)
	renderRepos(repos, jobTaskSpec.Properties.Envs)
	gitRepos, p4Repos := splitReposByType(repos)

	codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
	}

	// init git clone step
	gitStep := &commonmodels.StepTask{
		Name:     scanning.Name + "-git",
		JobName:  jobTask.Name,
		StepType: config.StepGit,
		Spec:     step.StepGitSpec{Repos: gitRepos, CodeHosts: codehosts},
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

	p4Step := &commonmodels.StepTask{
		Name:     scanning.Name + "-perforce",
		JobName:  jobTask.Name,
		StepType: config.StepPerforce,
		Spec:     step.StepP4Spec{Repos: p4Repos},
	}

	jobTaskSpec.Steps = append(jobTaskSpec.Steps, p4Step)

	repoName := ""
	branch := ""
	if len(scanning.Repos) > 0 {
		if scanning.Repos[0].CheckoutPath != "" {
			repoName = scanning.Repos[0].CheckoutPath
		} else {
			repoName = scanningInfo.Repos[0].RepoName
		}

		branch = scanning.Repos[0].Branch
	}
	// init debug before step
	debugBeforeStep := &commonmodels.StepTask{
		Name:     scanning.Name + "-debug-before",
		JobName:  jobTask.Name,
		StepType: config.StepDebugBefore,
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)
	// init script step
	if scanningInfo.ScannerType == types.ScanningTypeSonar {
		scriptStep := &commonmodels.StepTask{
			JobName: jobTask.Name,
		}
		if scanningInfo.ScriptType == types.ScriptTypeShell || scanningInfo.ScriptType == "" {
			scriptStep.Name = scanning.Name + "-shell"
			scriptStep.StepType = config.StepShell
			scriptStep.Spec = &step.StepShellSpec{
				Scripts:     append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
				SkipPrepare: true,
			}
		} else if scanningInfo.ScriptType == types.ScriptTypeBatchFile {
			scriptStep.Name = scanning.Name + "-batchfile"
			scriptStep.StepType = config.StepBatchFile
			scriptStep.Spec = &step.StepBatchFileSpec{
				Scripts:     append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
				SkipPrepare: true,
			}
		} else if scanningInfo.ScriptType == types.ScriptTypePowerShell {
			scriptStep.Name = scanning.Name + "-powershell"
			scriptStep.StepType = config.StepPowerShell
			scriptStep.Spec = &step.StepPowerShellSpec{
				Scripts:     append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
				SkipPrepare: true,
			}
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)

		sonarInfo, err := commonrepo.NewSonarIntegrationColl().GetByID(context.TODO(), scanningInfo.SonarID)
		if err != nil {
			return nil, fmt.Errorf("failed to get sonar integration information to create scanning task, error: %s", err)

		}

		sonarLinkKeyVal := &commonmodels.KeyVal{
			Key: "SONAR_LINK",
		}
		projectKey := commonutil.RenderEnv(sonar.GetSonarProjectKeyFromConfig(scanningInfo.Parameter), jobTaskSpec.Properties.Envs)
		sonarBranch := commonutil.RenderEnv(sonar.GetSonarBranchFromConfig(scanningInfo.Parameter), append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{Key: "branch", Value: branch}))
		resultAddr, err := sonar.GetSonarAddress(sonarInfo.ServerAddress, projectKey, sonarBranch)
		if err != nil {
			log.Errorf("failed to get sonar address, project: %s, branch: %s,, error: %s", projectKey, sonarBranch, err)
		}
		sonarLinkKeyVal.Value = resultAddr

		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, sonarLinkKeyVal)
		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
			Key:          "SONAR_TOKEN",
			Value:        sonarInfo.Token,
			IsCredential: true,
		})

		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.Envs, &commonmodels.KeyVal{
			Key:   "SONAR_URL",
			Value: sonarInfo.ServerAddress,
		})

		if scanningInfo.EnableScanner {
			sonarScriptStep := &commonmodels.StepTask{
				JobName: jobTask.Name,
			}
			if scanningInfo.ScriptType == types.ScriptTypeShell || scanningInfo.ScriptType == "" {
				sonarConfig := fmt.Sprintf("sonar.login=%s\nsonar.host.url=%s\n%s", sonarInfo.Token, sonarInfo.ServerAddress, scanningInfo.Parameter)
				sonarConfig = strings.ReplaceAll(sonarConfig, "$branch", branch)
				sonarScript := fmt.Sprintf("set -e\ncd %s\ncat > sonar-project.properties << EOF\n%s\nEOF\nsonar-scanner", repoName, commonutil.RenderEnv(sonarConfig, jobTaskSpec.Properties.Envs))

				sonarScriptStep.Name = scanning.Name + "-sonar-shell"
				sonarScriptStep.StepType = config.StepShell
				sonarScriptStep.Spec = &step.StepShellSpec{
					Scripts:     strings.Split(replaceWrapLine(sonarScript), "\n"),
					SkipPrepare: true,
				}
			} else if scanningInfo.ScriptType == types.ScriptTypeBatchFile {
				sonarScript := fmt.Sprintf("@echo off\nsetlocal enabledelayedexpansion\ncd %s\n\n", repoName)
				sonarScript += "(\n"

				sonarConfig := fmt.Sprintf("sonar.login=%s\nsonar.host.url=%s\n%s", sonarInfo.Token, sonarInfo.ServerAddress, scanningInfo.Parameter)
				sonarConfig = strings.ReplaceAll(sonarConfig, "$branch", branch)
				sonarConfig = commonutil.RenderEnv(sonarConfig, jobTaskSpec.Properties.Envs)
				sonarConfigArr := strings.Split(sonarConfig, "\n")
				for _, config := range sonarConfigArr {
					sonarScript += fmt.Sprintf("echo %s\n", config)
				}

				sonarScript += "\n) > sonar-project.properties\n\nsonar-scanner\n\nendlocal"
				sonarScriptStep.Name = scanning.Name + "-sonar-batchfile"
				sonarScriptStep.StepType = config.StepBatchFile
				sonarScriptStep.Spec = &step.StepBatchFileSpec{
					Scripts:     strings.Split(replaceWrapLine(sonarScript), "\n"),
					SkipPrepare: true,
				}
			} else if scanningInfo.ScriptType == types.ScriptTypePowerShell {
				sonarScript := fmt.Sprintf("Set-StrictMode -Version Latest\nSet-Location -Path \"%s\"\n", repoName)
				sonarScript += "@\"\n"

				sonarConfig := fmt.Sprintf("sonar.login=%s\nsonar.host.url=%s\n%s", sonarInfo.Token, sonarInfo.ServerAddress, scanningInfo.Parameter)
				sonarConfig = strings.ReplaceAll(sonarConfig, "$branch", branch)
				sonarConfig = commonutil.RenderEnv(sonarConfig, jobTaskSpec.Properties.Envs)
				sonarConfigArr := strings.Split(sonarConfig, "\n")
				for _, config := range sonarConfigArr {
					sonarScript += fmt.Sprintf("%s\n", config)
				}

				sonarScript += "\"@ | Out-File -FilePath \"sonar-project.properties\" -Encoding UTF8\n\nsonar-scanner"
				sonarScriptStep.Name = scanning.Name + "-sonar-powershell"
				sonarScriptStep.StepType = config.StepPowerShell
				sonarScriptStep.Spec = &step.StepPowerShellSpec{
					Scripts:     strings.Split(replaceWrapLine(sonarScript), "\n"),
					SkipPrepare: true,
				}
			}

			jobTaskSpec.Steps = append(jobTaskSpec.Steps, sonarScriptStep)

		}

		sonarGetMetricsStep := &commonmodels.StepTask{
			Name:     scanning.Name + "-sonar-get-metrics",
			JobName:  jobTask.Name,
			JobKey:   jobTask.Key,
			StepType: config.StepSonarGetMetrics,
			Spec: &step.StepSonarGetMetricsSpec{
				ProjectKey:       projectKey,
				Branch:           sonarBranch,
				Parameter:        scanningInfo.Parameter,
				CheckDir:         repoName,
				SonarToken:       sonarInfo.Token,
				SonarServer:      sonarInfo.ServerAddress,
				CheckQualityGate: scanningInfo.CheckQualityGate,
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, sonarGetMetricsStep)

		if scanningInfo.CheckQualityGate {
			sonarChekStep := &commonmodels.StepTask{
				Name:     scanning.Name + "-sonar-check",
				JobName:  jobTask.Name,
				JobKey:   jobTask.Key,
				StepType: config.StepSonarCheck,
				Spec: &step.StepSonarCheckSpec{
					ProjectKey:  projectKey,
					Parameter:   scanningInfo.Parameter,
					CheckDir:    repoName,
					SonarToken:  sonarInfo.Token,
					SonarServer: sonarInfo.ServerAddress,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, sonarChekStep)
		}
	} else {
		scriptStep := &commonmodels.StepTask{
			JobName: jobTask.Name,
		}
		if scanningInfo.ScriptType == types.ScriptTypeShell || scanningInfo.ScriptType == "" {
			scriptStep.Name = scanning.Name + "-shell"
			scriptStep.StepType = config.StepShell
			scriptStep.Spec = &step.StepShellSpec{
				Scripts: append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
			}
		} else if scanningInfo.ScriptType == types.ScriptTypeBatchFile {
			scriptStep.Name = scanning.Name + "-batchfile"
			scriptStep.StepType = config.StepBatchFile
			scriptStep.Spec = &step.StepBatchFileSpec{
				Scripts: append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
			}
		} else if scanningInfo.ScriptType == types.ScriptTypePowerShell {
			scriptStep.Name = scanning.Name + "-powershell"
			scriptStep.StepType = config.StepPowerShell
			scriptStep.Spec = &step.StepPowerShellSpec{
				Scripts: append(strings.Split(replaceWrapLine(scanningInfo.Script), "\n"), outputScript(scanningInfo.Outputs, jobTask.Infrastructure)...),
			}
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)
	}

	destDir := "/tmp"
	if scanningInfo.ScriptType == types.ScriptTypeBatchFile {
		destDir = "%TMP%"
	} else if scanningInfo.ScriptType == types.ScriptTypePowerShell {
		destDir = "%TMP%"
	}
	// init scanning result storage step
	if len(scanningInfo.AdvancedSetting.ArtifactPaths) > 0 {
		tarArchiveStep := &commonmodels.StepTask{
			Name:      config.ScanningJobArchiveResultStepName,
			JobName:   jobTask.Name,
			StepType:  config.StepTarArchive,
			Onfailure: true,
			Spec: &step.StepTarArchiveSpec{
				ResultDirs: scanningInfo.AdvancedSetting.ArtifactPaths,
				S3DestDir:  path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "scanning-result"),
				FileName:   setting.ArtifactResultOut,
				DestDir:    destDir,
			},
		}
		if len(scanningInfo.AdvancedSetting.ArtifactPaths) > 1 || scanningInfo.AdvancedSetting.ArtifactPaths[0] != "" {
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
		}
	}

	// init debug after step
	debugAfterStep := &commonmodels.StepTask{
		Name:     scanning.Name + "-debug-after",
		JobName:  jobTask.Name,
		StepType: config.StepDebugAfter,
	}

	// init object cache step
	if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
		cacheDir := "/workspace"
		if jobTaskSpec.Properties.CacheDirType == types.UserDefinedCacheDir {
			cacheDir = jobTaskSpec.Properties.CacheUserDir
		}
		tarArchiveStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", scanning.Name, "tar-archive"),
			JobName:  jobTask.Name,
			StepType: config.StepTarArchive,
			Spec: step.StepTarArchiveSpec{
				FileName:     setting.ScanningOSSCacheFileName,
				ResultDirs:   []string{"."},
				AbsResultDir: true,
				TarDir:       cacheDir,
				ChangeTarDir: true,
				S3DestDir:    getScanningJobCacheObjectPath(j.workflow.Name, scanning.Name),
				IgnoreErr:    true,
				S3Storage:    modelS3toS3(cacheS3),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
	}

	jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)

	return jobTask, nil
}

func (j ScanningJobController) getReferredJobTargets(jobName string) ([]*commonmodels.ServiceTestTarget, error) {
	servicetargets := []*commonmodels.ServiceTestTarget{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != j.jobSpec.JobName {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return servicetargets, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					servicetargets = append(servicetargets, &commonmodels.ServiceTestTarget{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
					})
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return servicetargets, err
				}
				for _, distribute := range distributeSpec.Targets {
					servicetargets = append(servicetargets, &commonmodels.ServiceTestTarget{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
					})
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return servicetargets, err
				}
				for _, svc := range deploySpec.Services {
					for _, module := range svc.Modules {
						servicetargets = append(servicetargets, &commonmodels.ServiceTestTarget{
							ServiceName:   svc.ServiceName,
							ServiceModule: module.ServiceModule,
						})
					}
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigScanning {
				scanningSpec := &commonmodels.ZadigScanningJobSpec{}
				if err := commonmodels.IToi(job.Spec, scanningSpec); err != nil {
					return servicetargets, err
				}
				scanTargets := make([]*commonmodels.ServiceTestTarget, 0)
				for _, svc := range scanningSpec.ServiceAndScannings {
					scanTargets = append(scanTargets, &commonmodels.ServiceTestTarget{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					})
				}
				servicetargets = scanTargets
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigTesting {
				testingSpec := &commonmodels.ZadigTestingJobSpec{}
				if err := commonmodels.IToi(job.Spec, testingSpec); err != nil {
					return servicetargets, err
				}
				testTargets := make([]*commonmodels.ServiceTestTarget, 0)
				for _, svc := range testingSpec.ServiceAndTests {
					testTargets = append(testTargets, &commonmodels.ServiceTestTarget{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					})
				}
				servicetargets = testTargets
				return servicetargets, nil
			}
			if job.JobType == config.JobFreestyle {
				deploySpec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return servicetargets, err
				}
				if deploySpec.FreestyleJobType != config.ServiceFreeStyleJobType {
					return servicetargets, fmt.Errorf("freestyle job type %s not supported in reference", deploySpec.FreestyleJobType)
				}
				for _, svc := range deploySpec.Services {
					target := &commonmodels.ServiceTestTarget{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					}
					servicetargets = append(servicetargets, target)
				}
				return servicetargets, nil
			}
		}
	}
	return nil, fmt.Errorf("ScanningJob: refered job %s not found", jobName)
}

func fillScanningDetail(moduleScanning *commonmodels.Scanning) error {
	if moduleScanning.TemplateID == "" {
		return nil
	}
	templateInfo, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{
		ID: moduleScanning.TemplateID,
	})
	if err != nil {
		return fmt.Errorf("failed to find scanning template with id: %s, err: %s", moduleScanning.TemplateID, err)
	}

	moduleScanning.Infrastructure = templateInfo.Infrastructure
	moduleScanning.VMLabels = templateInfo.VMLabels
	moduleScanning.ScannerType = templateInfo.ScannerType
	moduleScanning.EnableScanner = templateInfo.EnableScanner
	moduleScanning.ImageID = templateInfo.ImageID
	moduleScanning.SonarID = templateInfo.SonarID
	moduleScanning.Installs = templateInfo.Installs
	moduleScanning.Parameter = templateInfo.Parameter
	moduleScanning.Envs = commonservice.MergeBuildEnvs(templateInfo.Envs.ToRuntimeList(), moduleScanning.Envs.ToRuntimeList()).ToKVList()
	moduleScanning.ScriptType = templateInfo.ScriptType
	moduleScanning.Script = templateInfo.Script
	moduleScanning.AdvancedSetting = templateInfo.AdvancedSetting
	moduleScanning.CheckQualityGate = templateInfo.CheckQualityGate

	return nil
}

func getScanningJobVariables(repos []*types.Repository, taskID int64, project, workflowName, workflowDisplayName, infrastructure, scanningType, serviceName, serviceModule, scanningName string) []*commonmodels.KeyVal {
	ret := []*commonmodels.KeyVal{}

	// basic envs
	ret = append(ret, prepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)
	// repo envs
	ret = append(ret, getReposVariables(repos)...)

	ret = append(ret, &commonmodels.KeyVal{Key: "SCANNING_TYPE", Value: scanningType, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: serviceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: serviceModule, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SCANNING_NAME", Value: scanningName, IsCredential: false})

	return ret
}

func ensureScanningOutputs(outputs []*commonmodels.Output) []*commonmodels.Output {
	keyMap := map[string]struct{}{}
	for _, output := range outputs {
		keyMap[output.Name] = struct{}{}
	}
	if _, ok := keyMap[setting.WorkflowScanningJobOutputKey]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: setting.WorkflowScanningJobOutputKey,
		})
	}
	if _, ok := keyMap[setting.WorkflowScanningJobOutputKeyProject]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: setting.WorkflowScanningJobOutputKeyProject,
		})
	}
	if _, ok := keyMap[setting.WorkflowScanningJobOutputKeyBranch]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: setting.WorkflowScanningJobOutputKeyBranch,
		})
	}
	return outputs
}

func getScanningJobCacheObjectPath(workflowName, scanningName string) string {
	return fmt.Sprintf("%s/cache/%s", workflowName, scanningName)
}
