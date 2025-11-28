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
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostdb "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type VMDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigVMDeployJobSpec
}

func CreateVMDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigVMDeployJobSpec)
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

	return VMDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j VMDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j VMDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j VMDeployJobController) Validate(isExecution bool) error {
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

func (j VMDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigVMDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.S3StorageID = currJobSpec.S3StorageID
	// source is a bit tricky: if the saved args has a source of fromjob, but it has been change to runtime in the config
	// we need to not only update its source but also set services to empty slice.
	if j.jobSpec.Source == config.SourceFromJob && currJobSpec.Source == config.SourceRuntime {
		j.jobSpec.ServiceAndVMDeploys = make([]*commonmodels.ServiceAndVMDeploy, 0)
	}
	j.jobSpec.Source = currJobSpec.Source

	if j.jobSpec.Source == config.SourceFromJob {
		j.jobSpec.JobName = currJobSpec.JobName
		j.jobSpec.OriginJobName = currJobSpec.OriginJobName
	}

	newOptions, err := generateVMDeployServiceInfo(j.workflow.Project, currJobSpec.Env, currJobSpec.ServiceAndVMDeploysOptions, ticket)
	if err != nil {
		log.Errorf("failed to generate deployable vm service for env: %s, project: %s, error: %s", currJobSpec.Env, j.workflow.Project, err)
		return err
	}
	newOptionMap := make(map[string]*commonmodels.ServiceAndVMDeploy)
	for _, service := range newOptions {
		newOptionMap[service.ServiceName] = service
	}

	newSelection := make([]*commonmodels.ServiceAndVMDeploy, 0)

	if useUserInput {
		for _, configuredSelection := range j.jobSpec.ServiceAndVMDeploys {
			if _, ok := newOptionMap[configuredSelection.ServiceName]; !ok {
				continue
			}

			newSelection = append(newSelection, &commonmodels.ServiceAndVMDeploy{
				KeyVals:            applyKeyVals(newOptionMap[configuredSelection.ServiceName].KeyVals, configuredSelection.KeyVals, true),
				Repos:              applyRepos(newOptionMap[configuredSelection.ServiceName].Repos, configuredSelection.Repos),
				DeployArtifactType: configuredSelection.DeployArtifactType,
				ServiceName:        configuredSelection.ServiceName,
				ServiceModule:      configuredSelection.ServiceModule,
				ArtifactURL:        configuredSelection.ArtifactURL,
				FileName:           configuredSelection.FileName,
				Image:              configuredSelection.Image,
				TaskID:             configuredSelection.TaskID,
				WorkflowType:       configuredSelection.WorkflowType,
				WorkflowName:       configuredSelection.WorkflowName,
				JobTaskName:        configuredSelection.JobTaskName,
			})
		}
	}

	newDefault := make([]*commonmodels.ServiceAndVMDeploy, 0)
	for _, configuredDefault := range j.jobSpec.DefaultServiceAndVMDeploys {
		// if service is deleted, remove it from the build default
		_, ok := newOptionMap[configuredDefault.ServiceName]
		if !ok {
			continue
		}

		configuredDefault.KeyVals = newOptionMap[configuredDefault.ServiceName].KeyVals
		configuredDefault.DeployName = newOptionMap[configuredDefault.ServiceName].DeployName
		configuredDefault.DeployArtifactType = newOptionMap[configuredDefault.ServiceName].DeployArtifactType
		configuredDefault.Repos = newOptionMap[configuredDefault.ServiceName].Repos
		newDefault = append(newDefault, configuredDefault)
	}

	j.jobSpec.DefaultServiceAndVMDeploys = newDefault
	j.jobSpec.ServiceAndVMDeploys = newSelection
	j.jobSpec.ServiceAndVMDeploysOptions = newOptions
	return nil
}

func (j VMDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	// there are no production environment for vm projects now
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:                j.workflow.Project,
		IsSortByProductName: true,
		Production:          util.GetBoolPointer(false),
	})

	if err != nil {
		log.Errorf("failed to list environments for project: %s, error: %s", j.workflow.Project, err)
		return err
	}

	envOptions := make([]*commonmodels.ZadigVMDeployEnvInformation, 0)

	for _, env := range envs {
		if !ticket.IsAllowedEnv(j.workflow.Project, env.EnvName) {
			continue
		}

		info, err := generateVMDeployServiceInfo(j.workflow.Project, env.EnvName, j.jobSpec.ServiceAndVMDeploysOptions, ticket)
		if err != nil {
			log.Errorf("failed to generate service deploy info for project: %s, error: %s", j.workflow.Project, err)
			return err
		}

		envOptions = append(envOptions, &commonmodels.ZadigVMDeployEnvInformation{
			Env:      env.EnvName,
			Services: info,
		})
	}

	j.jobSpec.EnvOptions = envOptions
	return nil
}

func (j VMDeployJobController) ClearOptions() {
	j.jobSpec.EnvOptions = make([]*commonmodels.ZadigVMDeployEnvInformation, 0)
}

func (j VMDeployJobController) ClearSelection() {
	j.jobSpec.ServiceAndVMDeploys = make([]*commonmodels.ServiceAndVMDeploy, 0)
}

func (j VMDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	envName := strings.ReplaceAll(j.jobSpec.Env, setting.FixedValueMark, "")
	_, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", envName)
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		return resp, fmt.Errorf("list private keys error: %v", err)
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("list project %s's services error: %v", j.workflow.Project, err)
	}
	serviceMap := map[string]*commonmodels.Service{}
	for _, service := range services {
		serviceMap[service.ServiceName] = service
	}

	var s3Storage *commonmodels.S3Storage
	originS3StorageSubfolder := ""
	// get deploy info from previous build job
	if j.jobSpec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}

		referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		targets, err := j.getReferredJobTargets(referredJob, int(taskID), j.jobSpec.ServiceAndVMDeploys)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", referredJob, err)
		}

		s3Storage, err = commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return resp, fmt.Errorf("find default s3 storage error: %v", err)
		}
		originS3StorageSubfolder = s3Storage.Subfolder
		// clear service and image list to prevent old data from remaining
		j.jobSpec.ServiceAndVMDeploys = targets
		j.jobSpec.S3StorageID = s3Storage.ID.Hex()
	} else {
		s3Storage, err = commonrepo.NewS3StorageColl().Find(j.jobSpec.S3StorageID)
		if err != nil {
			return resp, fmt.Errorf("find s3 storage id: %s, error: %v", j.jobSpec.S3StorageID, err)
		}
		originS3StorageSubfolder = s3Storage.Subfolder
	}

	var registry *commonmodels.RegistryNamespace
	if j.jobSpec.DockerRegistryID != "" {
		registry, err = commonservice.FindRegistryById(j.jobSpec.DockerRegistryID, true, log.SugaredLogger())
		if err != nil {
			return resp, fmt.Errorf("find registry: %s error: %v", j.jobSpec.DockerRegistryID, err)
		}
	}

	buildSvc := commonservice.NewBuildService()
	for jobSubTaskID, vmDeployInfo := range j.jobSpec.ServiceAndVMDeploys {
		s3Storage.Subfolder = originS3StorageSubfolder

		service, ok := serviceMap[vmDeployInfo.ServiceName]
		if !ok {
			return resp, fmt.Errorf("service %s not found", vmDeployInfo.ServiceName)
		}
		buildInfo, err := buildSvc.GetBuild(service.BuildName, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule)
		if err != nil {
			return resp, fmt.Errorf("get build info for service %s error: %v", vmDeployInfo.ServiceName, err)
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(buildInfo.PreDeploy.ImageID)
		if err != nil {
			return resp, fmt.Errorf("find base image: %s error: %v", buildInfo.PreBuild.ImageID, err)
		}

		customEnvs := applyKeyVals(buildInfo.PreDeploy.Envs.ToRuntimeList(), vmDeployInfo.KeyVals, true).ToKVList()
		renderedCustomEnv, err := replaceServiceAndModules(customEnvs, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule)
		if err != nil {
			return nil, fmt.Errorf("failed to render service variables, error: %v", err)
		}

		paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
		envs := mergeKeyVals(renderedCustomEnv, paramEnvs)
		renderedEnv, err := replaceServiceAndModules(envs, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule)
		if err != nil {
			return nil, fmt.Errorf("failed to render service variables, error: %v", err)
		}

		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule),
			DisplayName: genJobDisplayName(j.name, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule),
			OriginName:  j.name,
			JobInfo: map[string]string{
				"service_name":   vmDeployInfo.ServiceName,
				"service_module": vmDeployInfo.ServiceModule,
				JobNameKey:       j.name,
			},
			JobType:        string(config.JobZadigVMDeploy),
			Spec:           jobTaskSpec,
			Timeout:        int64(buildInfo.Timeout),
			Infrastructure: buildInfo.DeployInfrastructure,
			VMLabels:       buildInfo.DeployVMLabels,
			ErrorPolicy:    j.errorPolicy,
			ExecutePolicy:  j.executePolicy,
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:         int64(timeout),
			ResourceRequest: buildInfo.PreBuild.ResReq,
			ResReqSpec:      buildInfo.PreBuild.ResReqSpec,
			CustomEnvs:      renderedCustomEnv,
			ClusterID:       buildInfo.PreBuild.ClusterID,
			StrategyID:      buildInfo.PreBuild.StrategyID,
			BuildOS:         basicImage.Value,
			ImageFrom:       buildInfo.PreDeploy.ImageFrom,
			ServiceName:     vmDeployInfo.ServiceName,
		}

		initShellScripts := []string{}
		vmDeployVars := []*commonmodels.KeyVal{}
		tmpVmDeployVars := getVMDeployJobVariables(vmDeployInfo, buildInfo, taskID, j.jobSpec.Env, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, jobTask.Infrastructure, vms, services, registry, log.SugaredLogger())
		for _, kv := range tmpVmDeployVars {
			if strings.HasSuffix(kv.Key, "_PK_CONTENT") {
				name := strings.TrimSuffix(kv.Key, "_PK_CONTENT")
				vmDeployVars = append(vmDeployVars, &commonmodels.KeyVal{Key: name + "_PK", Value: "/tmp/" + name + "_PK", IsCredential: false})

				initShellScripts = append(initShellScripts, "echo \""+kv.Value+"\" > /tmp/"+name+"_PK")
				initShellScripts = append(initShellScripts, "chmod 600 /tmp/"+name+"_PK")
			} else {
				vmDeployVars = append(vmDeployVars, kv)
			}
		}

		jobTaskSpec.Properties.Envs = append(renderedEnv, vmDeployVars...)
		jobTaskSpec.Properties.UseHostDockerDaemon = buildInfo.PreBuild.UseHostDockerDaemon
		jobTaskSpec.Properties.CacheEnable = false

		// init tools install step
		tools := []*step.Tool{}
		for _, tool := range buildInfo.PreDeploy.Installs {
			tools = append(tools, &step.Tool{
				Name:    tool.Name,
				Version: tool.Version,
			})
		}
		toolInstallStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", vmDeployInfo.ServiceName, "tool-install"),
			JobName:  jobTask.Name,
			StepType: config.StepTools,
			Spec:     step.StepToolInstallSpec{Installs: tools},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, toolInstallStep)
		// init git clone step
		repos := applyRepos(buildInfo.DeployRepos, vmDeployInfo.Repos)
		renderRepos(repos, jobTaskSpec.Properties.Envs)
		gitRepos, p4Repos := splitReposByType(repos)

		codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
		if err != nil {
			return nil, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
		}

		gitStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: gitRepos, CodeHosts: codehosts},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

		p4Step := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-perforce",
			JobName:  jobTask.Name,
			StepType: config.StepPerforce,
			Spec:     step.StepP4Spec{Repos: p4Repos},
		}

		jobTaskSpec.Steps = append(jobTaskSpec.Steps, p4Step)

		paths := make([]string, 0)
		if s3Storage.Subfolder != "" {
			paths = append(paths, s3Storage.Subfolder)
		}
		paths = append(paths, []string{vmDeployInfo.WorkflowName, strconv.Itoa(vmDeployInfo.TaskID), vmDeployInfo.JobTaskName, "archive"}...)

		if buildInfo.PostBuild.FileArchive != nil {
			// init download artifact step
			downloadArtifactStep := &commonmodels.StepTask{
				Name:     vmDeployInfo.ServiceName + "-download-artifact",
				JobName:  jobTask.Name,
				StepType: config.StepDownloadArchive,
				Spec: step.StepDownloadArchiveSpec{
					FileName:   vmDeployInfo.FileName,
					DestDir:    "artifact",
					ObjectPath: strings.Join(paths, "/"),
					S3:         modelS3toS3(s3Storage),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArtifactStep)
		}
		// init debug before step
		debugBeforeStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-debug_before",
			JobName:  jobTask.Name,
			StepType: config.StepDebugBefore,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)
		// init shell step
		scripts := append([]string{}, initShellScripts...)
		scripts = append(scripts, strings.Split(replaceWrapLine(buildInfo.PMDeployScripts), "\n")...)
		scriptStep := &commonmodels.StepTask{
			JobName: jobTask.Name,
		}
		if buildInfo.PMDeployScriptType == types.ScriptTypeShell || buildInfo.PMDeployScriptType == "" {
			scriptStep.Name = vmDeployInfo.ServiceName + "-shell"
			scriptStep.StepType = config.StepShell
			scriptStep.Spec = &step.StepShellSpec{
				Scripts: scripts,
			}
		} else if buildInfo.PMDeployScriptType == types.ScriptTypeBatchFile {
			scriptStep.Name = vmDeployInfo.ServiceName + "-batchfile"
			scriptStep.StepType = config.StepBatchFile
			scriptStep.Spec = &step.StepBatchFileSpec{
				Scripts: scripts,
			}
		} else if buildInfo.PMDeployScriptType == types.ScriptTypePowerShell {
			scriptStep.Name = vmDeployInfo.ServiceName + "-powershell"
			scriptStep.StepType = config.StepPowerShell
			scriptStep.Spec = &step.StepPowerShellSpec{
				Scripts: scripts,
			}
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)
		// init debug after step
		debugAfterStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-debug_after",
			JobName:  jobTask.Name,
			StepType: config.StepDebugAfter,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)

		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j VMDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j VMDeployJobController) SetRepoCommitInfo() error {
	for _, vmDeploy := range j.jobSpec.ServiceAndVMDeploys {
		if err := setRepoInfo(vmDeploy.Repos); err != nil {
			return err
		}
	}

	return nil
}

func (j VMDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	resp = append(resp, &commonmodels.KeyVal{
		Key:          strings.Join([]string{"job", j.name, "envName"}, "."),
		Value:        j.jobSpec.Env,
		Type:         "string",
		IsCredential: false,
	})

	if getAggregatedVariables {
		services := make([]string, 0)
		pkgs := make([]string, 0)
		for _, svc := range j.jobSpec.ServiceAndVMDeploys {
			services = append(services, svc.ServiceName)
			pkgs = append(pkgs, svc.FileName)
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "PKG_FILES"}, "."),
			Value:        strings.Join(pkgs, ","),
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "SERVICES"}, "."),
			Value:        strings.Join(services, ","),
			Type:         "string",
			IsCredential: false,
		})
	}

	if getRuntimeVariables {
		for _, svc := range j.jobSpec.ServiceAndVMDeploys {
			targetKey := strings.Join([]string{j.name, svc.ServiceName, svc.ServiceModule}, ".")
			resp = append(resp, &commonmodels.KeyVal{
				Key:          strings.Join([]string{"job", targetKey, "status"}, "."),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})
		}
	}

	if getPlaceHolderVariables {
		jobKey := strings.Join([]string{j.name, "<SERVICE>", "<MODULE>"}, ".")
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", jobKey, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}

	return resp, nil
}

func (j VMDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	buildSvc := commonservice.NewBuildService()
	for _, vmDeploy := range j.jobSpec.ServiceAndVMDeploys {
		buildInfo, err := buildSvc.GetBuild(vmDeploy.DeployName, vmDeploy.ServiceName, vmDeploy.ServiceModule)
		if err != nil {
			log.Errorf("find vm deploy: %s error: %v", vmDeploy.DeployName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == vmDeploy.ServiceName && target.ServiceModule == vmDeploy.ServiceModule {
				resp = append(resp, applyRepos(buildInfo.DeployRepos, vmDeploy.Repos)...)
				break
			}
		}
	}
	return resp, nil
}

func (j VMDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j VMDeployJobController) IsServiceTypeJob() bool {
	return true
}

func (j VMDeployJobController) getReferredJobTargets(jobName string, taskID int, serviceAndVMDeploys []*commonmodels.ServiceAndVMDeploy) ([]*commonmodels.ServiceAndVMDeploy, error) {
	serviceAndVMDeployMap := map[string]*commonmodels.ServiceAndVMDeploy{}
	for _, serviceAndVMDeploy := range serviceAndVMDeploys {
		serviceAndVMDeployMap[serviceAndVMDeploy.ServiceName] = serviceAndVMDeploy
	}

	newServiceAndVMDeploys := make([]*commonmodels.ServiceAndVMDeploy, 0)
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != j.jobSpec.JobName {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return newServiceAndVMDeploys, err
				}
				for i, build := range buildSpec.ServiceAndBuilds {
					serviceAndVMDeploy := &commonmodels.ServiceAndVMDeploy{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
						FileName:      build.Package,
						Image:         build.Image,
						TaskID:        taskID,
						WorkflowName:  j.workflow.Name,
						WorkflowType:  config.WorkflowTypeV4,
						JobTaskName:   GenJobName(j.workflow, job.Name, i),
					}

					if serviceAndVMDeployMap[build.ServiceName] != nil {
						serviceAndVMDeploy.DeployName = serviceAndVMDeployMap[build.ServiceName].DeployName
						serviceAndVMDeploy.DeployArtifactType = serviceAndVMDeployMap[build.ServiceName].DeployArtifactType
						serviceAndVMDeploy.Repos = applyRepos(serviceAndVMDeployMap[build.ServiceName].Repos, build.Repos)
						serviceAndVMDeploy.KeyVals = serviceAndVMDeployMap[build.ServiceName].KeyVals
					}

					newServiceAndVMDeploys = append(newServiceAndVMDeploys, serviceAndVMDeploy)

					log.Infof("DeployJob ToJobs getOriginReferedJobTargets: workflow %s service %s, module %s, fileName %s",
						j.workflow.Name, build.ServiceName, build.ServiceModule, build.Package)
				}
				return newServiceAndVMDeploys, nil
			}
		}
	}
	return nil, fmt.Errorf("build job %s not found", jobName)
}

// generateVMDeployServiceInfo generated all deployable service and its corresponding data.
// currently it ignores the env service info, just gives all the service defined in the template.
func generateVMDeployServiceInfo(project, env string, serviceAndVMDeploys []*commonmodels.ServiceAndVMDeploy, ticket *commonmodels.ApprovalTicket) ([]*commonmodels.ServiceAndVMDeploy, error) {
	resp := make([]*commonmodels.ServiceAndVMDeploy, 0)

	environmentInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: env,
		Name:    project,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find env: %s in project: %s, error: %s", env, project, err)
	}

	svcAndVMDeployMap := map[string]*commonmodels.ServiceAndVMDeploy{}
	for _, serviceAndVMDeploy := range serviceAndVMDeploys {
		svcAndVMDeployMap[serviceAndVMDeploy.ServiceName] = serviceAndVMDeploy
	}

	svcs := environmentInfo.GetServiceMap()

	for _, svc := range svcs {
		if len(svcAndVMDeployMap) > 0 {
			if _, ok := svcAndVMDeployMap[svc.ServiceName]; !ok {
				continue
			}
		}

		templateSvc, err := commonrepo.NewServiceColl().Find(
			&commonrepo.ServiceFindOption{
				ServiceName: svc.ServiceName,
				ProductName: project,
				Revision:    svc.Revision,
			},
		)

		if err != nil {
			return nil, fmt.Errorf("failed to find service: %s in project: %s, error: %s", svc.ServiceName, project, err)
		}

		if !ticket.IsAllowedService(project, templateSvc.ServiceName, templateSvc.ServiceName) {
			continue
		}

		if templateSvc.BuildName == "" {
			return nil, fmt.Errorf("service %s in project %s has no deploy info", svc.ServiceName, project)
		}

		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: templateSvc.BuildName, ProductName: project})
		if err != nil {
			return nil, fmt.Errorf("can't find build %s in project %s, error: %v", templateSvc.BuildName, project, err)
		}

		repos := build.DeployRepos
		keyVals := build.PreDeploy.Envs.ToRuntimeList()
		if svcAndVMDeployMap[svc.ServiceName] != nil {
			repos = applyRepos(build.DeployRepos, svcAndVMDeployMap[svc.ServiceName].Repos)
			keyVals = applyKeyVals(build.PreDeploy.Envs.ToRuntimeList(), svcAndVMDeployMap[svc.ServiceName].KeyVals, false)
		}

		resp = append(resp, &commonmodels.ServiceAndVMDeploy{
			Repos:              repos,
			KeyVals:            keyVals,
			DeployName:         build.Name,
			DeployArtifactType: build.DeployArtifactType,
			ServiceName:        templateSvc.ServiceName,
			ServiceModule:      templateSvc.ServiceName,
		})
	}

	return resp, nil
}

// TODO: maybe use the get variables function
// this is for internal use only
func getVMDeployJobVariables(vmDeploy *commonmodels.ServiceAndVMDeploy, buildInfo *commonmodels.Build, taskID int64, envName, project, workflowName, workflowDisplayName, infrastructure string, vms []*commonmodels.PrivateKey, services []*commonmodels.Service, registry *commonmodels.RegistryNamespace, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, prepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)

	// registry envs
	if registry != nil {
		registryHost := strings.TrimPrefix(registry.RegAddr, "http://")
		registryHost = strings.TrimPrefix(registryHost, "https://")
		ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_HOST", Value: registryHost, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_NAMESPACE", Value: registry.Namespace, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_AK", Value: registry.AccessKey, IsCredential: false})
		ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_SK", Value: registry.SecretKey, IsCredential: true})
	}

	// repo envs
	ret = append(ret, getReposVariables(vmDeploy.Repos)...)

	// vm deploy specific envs
	ret = append(ret, &commonmodels.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: vmDeploy.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: vmDeploy.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: vmDeploy.ServiceModule, IsCredential: false})

	privateKeys := sets.String{}
	IDvmMap := map[string]*commonmodels.PrivateKey{}
	labelVMsMap := map[string][]*commonmodels.PrivateKey{}
	for _, vm := range vms {
		privateKeys.Insert(vm.Name)
		IDvmMap[vm.ID.Hex()] = vm
		labelVMsMap[vm.Label] = append(labelVMsMap[vm.Label], vm)
	}
	ret = append(ret, &commonmodels.KeyVal{Key: "AGENTS", Value: strings.Join(privateKeys.List(), ","), IsCredential: false})

	agentVMIDs := sets.String{}
	if len(buildInfo.SSHs) > 0 {
		// privateKeys := make([]*taskmodels.SSH, 0)
		for _, sshID := range buildInfo.SSHs {
			//私钥信息可能被更新，而构建中存储的信息是旧的，需要根据id获取最新的私钥信息
			latestKeyInfo, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: sshID})
			if err != nil || latestKeyInfo == nil {
				log.Errorf("PrivateKey.Find failed, id:%s, err:%s", sshID, err)
				continue
			}
			agentName := latestKeyInfo.Name
			userName := latestKeyInfo.UserName
			ip := latestKeyInfo.IP
			port := latestKeyInfo.Port
			if port == 0 {
				port = setting.PMHostDefaultPort
			}
			privateKey, err := base64.StdEncoding.DecodeString(latestKeyInfo.PrivateKey)
			if err != nil {
				log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
				continue
			}
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_PK_CONTENT", Value: string(privateKey), IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_USERNAME", Value: userName, IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_IP", Value: ip, IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
			agentVMIDs.Insert(sshID)
		}
	}

	envHostNamesMap := map[string][]string{}
	envHostIPsMap := map[string][]string{}
	addedHostIDs := sets.String{}
	for _, svc := range services {
		if svc.ServiceName != vmDeploy.ServiceName {
			continue
		}
		for _, envConfig := range svc.EnvConfigs {
			for _, hostID := range envConfig.HostIDs {
				if vm, ok := IDvmMap[hostID]; ok {
					if envName == envConfig.EnvName {
						envHostNamesMap[envConfig.EnvName] = append(envHostNamesMap[envConfig.EnvName], vm.Name)
						envHostIPsMap[envConfig.EnvName] = append(envHostIPsMap[envConfig.EnvName], vm.IP)
					}

					if agentVMIDs.Has(hostID) || addedHostIDs.Has(hostID) {
						continue
					}
					addedHostIDs.Insert(hostID)
					hostName := vm.Name
					userName := vm.UserName
					ip := vm.IP
					port := vm.Port
					if port == 0 {
						port = setting.PMHostDefaultPort
					}
					privateKey, err := base64.StdEncoding.DecodeString(vm.PrivateKey)
					if err != nil {
						log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
						continue
					}
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PK_CONTENT", Value: string(privateKey), IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_USERNAME", Value: userName, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_IP", Value: ip, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
				}
			}
			for _, label := range envConfig.Labels {
				for _, vm := range labelVMsMap[label] {
					if envName == envConfig.EnvName {
						envHostNamesMap[envConfig.EnvName] = append(envHostNamesMap[envConfig.EnvName], vm.Name)
						envHostIPsMap[envConfig.EnvName] = append(envHostIPsMap[envConfig.EnvName], vm.IP)
					}

					if agentVMIDs.Has(vm.ID.Hex()) || addedHostIDs.Has(vm.ID.Hex()) {
						continue
					}
					addedHostIDs.Insert(vm.ID.Hex())

					hostName := vm.Name
					userName := vm.UserName
					ip := vm.IP
					port := vm.Port
					if port == 0 {
						port = setting.PMHostDefaultPort
					}
					privateKey := vm.PrivateKey
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PK_CONTENT", Value: privateKey, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_USERNAME", Value: userName, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_IP", Value: ip, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
				}
			}
		}
	}
	// env host ips
	for envName, HostIPs := range envHostIPsMap {
		ret = append(ret, &commonmodels.KeyVal{Key: envName + "_HOST_IPs", Value: strings.Join(HostIPs, ","), IsCredential: false})
	}
	// env host names
	for envName, names := range envHostNamesMap {
		ret = append(ret, &commonmodels.KeyVal{Key: envName + "_HOST_NAMEs", Value: strings.Join(names, ","), IsCredential: false})
	}

	if infrastructure != setting.JobVMInfrastructure {
		ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "/workspace/artifact/" + vmDeploy.FileName, IsCredential: false})
	} else {
		if buildInfo.PMDeployScriptType == types.ScriptTypeShell || buildInfo.PMDeployScriptType == "" {
			ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "$WORKSPACE/artifact/" + vmDeploy.FileName, IsCredential: false})
		} else if buildInfo.PMDeployScriptType == types.ScriptTypeBatchFile {
			ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "%WORKSPACE%\\artifact\\" + vmDeploy.FileName, IsCredential: false})
		} else if buildInfo.PMDeployScriptType == types.ScriptTypePowerShell {
			ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "$env:WORKSPACE\\artifact\\" + vmDeploy.FileName, IsCredential: false})
		}
	}
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: vmDeploy.FileName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "IMAGE", Value: vmDeploy.Image, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DEPLOY_ARTIFACT_TYPE", Value: string(vmDeploy.DeployArtifactType), IsCredential: false})
	return ret
}

func vmRenderRepos(repos []*types.Repository, kvs []*commonmodels.KeyVal) []*types.Repository {
	for _, inputRepo := range repos {
		inputRepo.CheckoutPath = commonutil.RenderEnv(inputRepo.CheckoutPath, kvs)
		if inputRepo.RemoteName == "" {
			inputRepo.RemoteName = "origin"
		}
		if inputRepo.Source == types.ProviderOther {
			codeHostInfo, err := codehostdb.NewCodehostColl().GetCodeHostByID(inputRepo.CodehostID, false)
			if err == nil {
				inputRepo.PrivateAccessToken = codeHostInfo.PrivateAccessToken
				inputRepo.SSHKey = codeHostInfo.SSHKey
			}
		}
	}
	return repos
}
