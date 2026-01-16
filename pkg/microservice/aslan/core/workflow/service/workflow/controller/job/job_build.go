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
	"path"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	templ "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
	pkgutil "github.com/koderover/zadig/v2/pkg/util"
)

// TODO: Change note: ServiceAndBuilds field use to be the option field for the configuration, it has been
// moved to ServiceAndBuildsOptions field

// TODO2: Change note: KevVal in the serviceAndBuildOptions has been changed: added fixed

const (
	IMAGEKEY               = "IMAGE"
	IMAGETAGKEY            = "imageTag"
	PKGFILEKEY             = "PKG_FILE"
	BRANCHKEY              = "BRANCH"
	REPONAMEKEY            = "REPONAME"
	GITURLKEY              = "GITURL"
	COMMITIDKEY            = "COMMITID"
	PRE_MEGRE_BRANCHES_KEY = "PRE_MEGRE_BRANCHES"
)

type BuildJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigBuildJobSpec
}

func CreateBuildJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigBuildJobSpec)
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

	return BuildJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j BuildJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j BuildJobController) GetSpec() interface{} {
	return j.jobSpec
}

const (
	buildKeyTemplate = "%s++%s"
)

func (j BuildJobController) Validate(isExecution bool) error {
	optionMap := make(map[string]*commonmodels.ServiceAndBuild)
	for _, build := range j.jobSpec.ServiceAndBuildsOptions {
		key := fmt.Sprintf(buildKeyTemplate, build.ServiceName, build.ServiceModule)
		if _, ok := optionMap[key]; ok {
			return fmt.Errorf("duplicate service module in options field")
		}
		optionMap[key] = build
	}

	if isExecution {
		latestJob, err := j.workflow.FindJob(j.name, j.jobType)
		if err != nil {
			return fmt.Errorf("failed to find job: %s in workflow %s's latest config, error: %s", j.name, j.workflow.Name, err)
		}

		currJobSpec := new(commonmodels.ZadigBuildJobSpec)
		if err := commonmodels.IToi(latestJob.Spec, currJobSpec); err != nil {
			return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
		}

		optionMap = make(map[string]*commonmodels.ServiceAndBuild)

		for _, build := range currJobSpec.ServiceAndBuildsOptions {
			key := fmt.Sprintf(buildKeyTemplate, build.ServiceName, build.ServiceModule)
			optionMap[key] = build
		}

		for _, selectedBuild := range j.jobSpec.ServiceAndBuilds {
			key := fmt.Sprintf(buildKeyTemplate, selectedBuild.ServiceName, selectedBuild.ServiceModule)
			if _, ok := optionMap[key]; !ok {
				return fmt.Errorf("%s/%s is not in the configured service build list", selectedBuild.ServiceName, selectedBuild.ServiceModule)
			}
		}
	}

	return nil
}

func (j BuildJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return fmt.Errorf("failed to find job: %s in workflow %s's latest config, error: %s", j.name, j.workflow.Name, err)
	}

	latestJobSpec := new(commonmodels.ZadigBuildJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	userConfiguredService := make(map[string]*commonmodels.ServiceAndBuild)

	for _, service := range j.jobSpec.ServiceAndBuilds {
		key := fmt.Sprintf(buildKeyTemplate, service.ServiceName, service.ServiceModule)
		userConfiguredService[key] = service
	}

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return fmt.Errorf("get services map error: %v", err)
	}

	buildSvc := commonservice.NewBuildService()

	// update 2 fields according to the configuration: option and default. remove the service/module combination if the
	// service is deleted
	newOption := make([]*commonmodels.ServiceAndBuild, 0)
	newOptionMap := make(map[string]*commonmodels.ServiceAndBuild)
	for _, configuredBuild := range latestJobSpec.ServiceAndBuildsOptions {
		// if service is deleted, remove it from the build options
		service, ok := servicesMap[configuredBuild.ServiceName]
		if !ok {
			continue
		}

		buildInfo, err := buildSvc.GetBuild(configuredBuild.BuildName, configuredBuild.ServiceName, configuredBuild.ServiceModule)
		if err != nil {
			log.Errorf("configured build [%s] not found for [%s/%s], error: %s", configuredBuild.BuildName, configuredBuild.ServiceName, configuredBuild.ServiceModule, err)
			return fmt.Errorf("configured build [%s] not found for [%s/%s], error: %s", configuredBuild.BuildName, configuredBuild.ServiceName, configuredBuild.ServiceModule, err)
		}

		found := false
		for _, target := range buildInfo.Targets {
			if target.ServiceName == configuredBuild.ServiceName && target.ServiceModule == configuredBuild.ServiceModule {
				found = true
				break
			}
		}
		if !found {
			log.Errorf("configured build [%s] does not build [%s/%s], error: %s", configuredBuild.BuildName, configuredBuild.ServiceName, configuredBuild.ServiceModule, err)
			return fmt.Errorf("configured build [%s] does not build [%s/%s], error: %s", configuredBuild.BuildName, configuredBuild.ServiceName, configuredBuild.ServiceModule, err)
		}

		item := &commonmodels.ServiceAndBuild{
			ServiceName:      configuredBuild.ServiceName,
			ServiceModule:    configuredBuild.ServiceModule,
			BuildName:        configuredBuild.BuildName,
			Image:            configuredBuild.Image,
			Package:          configuredBuild.Package,
			ImageName:        configuredBuild.ImageName,
			ShareStorageInfo: configuredBuild.ShareStorageInfo,
			KeyVals:          applyKeyVals(buildInfo.PreBuild.Envs.ToRuntimeList(), configuredBuild.KeyVals, true),
			Repos:            applyRepos(buildInfo.Repos, configuredBuild.Repos),
		}

		for _, container := range service.Containers {
			if container.Name == configuredBuild.ServiceModule {
				item.ImageName = container.ImageName
				break
			}
		}

		newOption = append(newOption, item)
		key := fmt.Sprintf(buildKeyTemplate, configuredBuild.ServiceName, configuredBuild.ServiceModule)
		newOptionMap[key] = item
	}

	newDefault := make([]*commonmodels.ServiceAndBuild, 0)
	for _, configuredDefault := range latestJobSpec.DefaultServiceAndBuilds {
		// if service is deleted, remove it from the build default
		_, ok := servicesMap[configuredDefault.ServiceName]
		if !ok {
			continue
		}

		key := fmt.Sprintf(buildKeyTemplate, configuredDefault.ServiceName, configuredDefault.ServiceModule)
		option, ok := newOptionMap[key]

		newDefault = append(newDefault, option)
	}

	// for the ServiceAndBuildField, if we use user's input, check if it is allowed by the calculated options, if it is,
	// do a merge
	newSelection := make([]*commonmodels.ServiceAndBuild, 0)
	if useUserInput {
		for _, configuredSelection := range j.jobSpec.ServiceAndBuilds {
			// if service is deleted, remove it from the build default
			_, ok := servicesMap[configuredSelection.ServiceName]
			if !ok {
				continue
			}

			key := fmt.Sprintf(buildKeyTemplate, configuredSelection.ServiceName, configuredSelection.ServiceModule)
			// if the input is not allowed in the option, return error
			if _, ok := newOptionMap[key]; !ok {
				continue
			}

			item := &commonmodels.ServiceAndBuild{
				ServiceName:      configuredSelection.ServiceName,
				ServiceModule:    configuredSelection.ServiceModule,
				BuildName:        configuredSelection.BuildName,
				Image:            configuredSelection.Image,
				Package:          configuredSelection.Package,
				ImageName:        configuredSelection.ImageName,
				ShareStorageInfo: configuredSelection.ShareStorageInfo,
				KeyVals:          applyKeyVals(newOptionMap[key].KeyVals, configuredSelection.KeyVals, false),
				Repos:            applyRepos(newOptionMap[key].Repos, configuredSelection.Repos),
			}

			newSelection = append(newSelection, item)
		}
	} else {
		// else we do nothing since the configuration will always be empty
		// TODO: change this if required
	}

	j.jobSpec.DockerRegistryID = latestJobSpec.DockerRegistryID
	j.jobSpec.DefaultServiceAndBuilds = newDefault
	j.jobSpec.ServiceAndBuildsOptions = newOption
	j.jobSpec.ServiceAndBuilds = newSelection
	return nil
}

// SetOptions recalculate the option field using given ticket, if ticket is nil, it does nothing.
// note that this function alone will not update the build to the latest. Use update() to do that.
func (j BuildJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	if ticket == nil {
		return nil
	}

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return fmt.Errorf("get services map error: %v", err)
	}

	newBuilds := make([]*commonmodels.ServiceAndBuild, 0)
	for _, build := range j.jobSpec.ServiceAndBuilds {
		if !ticket.IsAllowedService(j.workflow.Project, build.ServiceName, build.ServiceModule) {
			continue
		}

		if _, ok := servicesMap[build.ServiceName]; !ok {
			continue
		}

		newBuilds = append(newBuilds, build)
	}

	j.jobSpec.ServiceAndBuildsOptions = newBuilds
	return nil
}

// ClearOptions removes the option field completely
func (j BuildJobController) ClearOptions() {
	j.jobSpec.ServiceAndBuildsOptions = make([]*commonmodels.ServiceAndBuild, 0)
}

func (j BuildJobController) ClearSelection() {
	j.jobSpec.ServiceAndBuilds = make([]*commonmodels.ServiceAndBuild, 0)
}

func (j BuildJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := make([]*commonmodels.JobTask, 0)
	registry, err := commonservice.FindRegistryById(j.jobSpec.DockerRegistryID, true, logger)
	if err != nil {
		return nil, fmt.Errorf("find docker registry: %s error: %v", j.jobSpec.DockerRegistryID, err)
	}

	registries, err := commonservice.ListRegistryNamespaces("", true, logger)
	if err != nil {
		return nil, err
	}

	defaultS3, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return nil, fmt.Errorf("find default s3 storage error: %v", err)
	}

	if j.jobSpec.Source == config.SourceFromJob {
		referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		targets, err := j.getReferredJobTargets(referredJob)
		if err != nil {
			return nil, fmt.Errorf("build job %s, get origin refered job: %s targets failed, err: %v", j.jobSpec.JobName, referredJob, err)
		}
		j.jobSpec.ServiceAndBuilds = targets
	}

	buildSvc := commonservice.NewBuildService()
	for jobSubTaskID, build := range j.jobSpec.ServiceAndBuilds {
		imageTag := commonservice.ReleaseCandidate(build.Repos, taskID, j.workflow.Project, build.ServiceModule, "", build.ImageName, "image")

		image := fmt.Sprintf("%s/%s", registry.RegAddr, imageTag)
		if len(registry.Namespace) > 0 {
			image = fmt.Sprintf("%s/%s/%s", registry.RegAddr, registry.Namespace, imageTag)
		}

		image = strings.TrimPrefix(image, "http://")
		image = strings.TrimPrefix(image, "https://")

		pkgFile := fmt.Sprintf("%s.tar.gz", commonservice.ReleaseCandidate(build.Repos, taskID, j.workflow.Project, build.ServiceModule, "", build.ImageName, "tar"))

		buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
		if err != nil {
			return nil, fmt.Errorf("find build: %s error: %v", build.BuildName, err)
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(buildInfo.PreBuild.ImageID)
		if err != nil {
			return nil, fmt.Errorf("find base image: %s error: %v", buildInfo.PreBuild.ImageID, err)
		}
		outputs := ensureBuiltInBuildOutputs(buildInfo.Outputs)
		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			JobInfo: map[string]string{
				"service_name":   build.ServiceName,
				"service_module": build.ServiceModule,
				JobNameKey:       j.name,
			},
			Key:            genJobKey(j.name, build.ServiceName, build.ServiceModule),
			Name:           GenJobName(j.workflow, j.name, jobSubTaskID),
			DisplayName:    genJobDisplayName(j.name, build.ServiceName, build.ServiceModule),
			OriginName:     j.name,
			JobType:        string(config.JobZadigBuild),
			Spec:           jobTaskSpec,
			Timeout:        int64(buildInfo.Timeout),
			Outputs:        outputs,
			Infrastructure: buildInfo.Infrastructure,
			VMLabels:       buildInfo.VMLabels,
			ErrorPolicy:    j.errorPolicy,
			ExecutePolicy:  j.executePolicy,
		}
		customEnvs := applyKeyVals(buildInfo.PreBuild.Envs.ToRuntimeList(), build.KeyVals, true).ToKVList()

		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:             int64(buildInfo.Timeout),
			ResourceRequest:     buildInfo.PreBuild.ResReq,
			ResReqSpec:          buildInfo.PreBuild.ResReqSpec,
			CustomEnvs:          customEnvs,
			ClusterID:           buildInfo.PreBuild.ClusterID,
			StrategyID:          buildInfo.PreBuild.StrategyID,
			BuildOS:             basicImage.Value,
			ImageFrom:           buildInfo.PreBuild.ImageFrom,
			Registries:          registries,
			ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, build.ShareStorageInfo, j.workflow.Name, taskID),
			CustomLabels:        buildInfo.PreBuild.CustomLabels,
			CustomAnnotations:   buildInfo.PreBuild.CustomAnnotations,
			EnablePrivileged:    buildInfo.EnablePrivilegedMode,
		}

		paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
		envs := mergeKeyVals(jobTaskSpec.Properties.CustomEnvs, paramEnvs)
		jobTaskSpec.Properties.Envs = append(envs, getBuildJobVariables(build, taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, image, pkgFile, jobTask.Infrastructure, registry, logger)...)
		jobTaskSpec.Properties.UseHostDockerDaemon = buildInfo.PreBuild.UseHostDockerDaemon

		if buildInfo.PreBuild != nil && buildInfo.PreBuild.Storages != nil && buildInfo.PreBuild.Storages.Enabled {
			if len(buildInfo.PreBuild.Storages.StoragesProperties) > 0 {
				newStorages := make([]*types.NFSProperties, 0)
				for _, storage := range buildInfo.PreBuild.Storages.StoragesProperties {
					newStorage := &types.NFSProperties{}
					err = pkgutil.DeepCopy(newStorage, storage)
					if err != nil {
						return nil, fmt.Errorf("failed to deep copy storage: %v", err)
					}

					newStorage.MountPath = commonutil.RenderEnv(storage.MountPath, jobTaskSpec.Properties.Envs)
					newStorage.Subpath = commonutil.RenderEnv(storage.Subpath, jobTaskSpec.Properties.Envs)
					newStorages = append(newStorages, newStorage)
				}

				jobTaskSpec.Properties.Storages = newStorages
			}
		}

		cacheS3 := &commonmodels.S3Storage{}
		if jobTask.Infrastructure == setting.JobVMInfrastructure {
			jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
		} else {
			clusterInfo, err := commonrepo.NewK8SClusterColl().Get(buildInfo.PreBuild.ClusterID)
			if err != nil {
				return nil, fmt.Errorf("find cluster: %s error: %v", buildInfo.PreBuild.ClusterID, err)
			}

			if clusterInfo.Cache.MediumType == "" {
				jobTaskSpec.Properties.CacheEnable = false
			} else {
				// set job task cache equal to cluster cache
				jobTaskSpec.Properties.Cache = clusterInfo.Cache
				jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
				jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
				jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
			}

			if jobTaskSpec.Properties.CacheEnable {
				jobTaskSpec.Properties.CacheUserDir = commonutil.RenderEnv(jobTaskSpec.Properties.CacheUserDir, jobTaskSpec.Properties.Envs)
				if jobTaskSpec.Properties.Cache.MediumType == types.NFSMedium {
					jobTaskSpec.Properties.Cache.NFSProperties.Subpath = commonutil.RenderEnv(jobTaskSpec.Properties.Cache.NFSProperties.Subpath, jobTaskSpec.Properties.Envs)
				} else if jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
					cacheS3, err = commonrepo.NewS3StorageColl().Find(jobTaskSpec.Properties.Cache.ObjectProperties.ID)
					if err != nil {
						return nil, fmt.Errorf("find cache s3 storage: %s error: %v", jobTaskSpec.Properties.Cache.ObjectProperties.ID, err)
					}

				}
			}
		}

		// for other job refer current latest image.
		build.Image = job.GetJobOutputKey(jobTask.Key, "IMAGE")
		build.Package = job.GetJobOutputKey(jobTask.Key, "PKG_FILE")
		log.Infof("BuildJob ToJobs %d: workflow %s service %s, module %s, image %s, package %s",
			taskID, j.workflow.Name, build.ServiceName, build.ServiceModule, build.Image, build.Package)

		// init tools install step
		tools := []*step.Tool{}
		for _, tool := range buildInfo.PreBuild.Installs {
			tools = append(tools, &step.Tool{
				Name:    tool.Name,
				Version: tool.Version,
			})
		}
		toolInstallStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", build.ServiceName, "tool-install"),
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
				Name:     fmt.Sprintf("%s-%s", build.ServiceName, "download-archive"),
				JobName:  jobTask.Name,
				StepType: config.StepDownloadArchive,
				Spec: step.StepDownloadArchiveSpec{
					UnTar:      true,
					IgnoreErr:  true,
					FileName:   setting.BuildOSSCacheFileName,
					ObjectPath: getBuildJobCacheObjectPath(j.workflow.Name, build.ServiceName, build.ServiceModule),
					DestDir:    cacheDir,
					S3:         modelToS3StepSpec(cacheS3),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArchiveStep)
		}

		codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
		if err != nil {
			return nil, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
		}

		// init git clone step
		repos := applyRepos(buildInfo.Repos, build.Repos)
		renderRepos(repos, jobTaskSpec.Properties.Envs)

		gitRepos, p4Repos := splitReposByType(repos)
		gitStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec: step.StepGitSpec{
				CodeHosts: codehosts,
				Repos:     gitRepos,
			},
		}

		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

		p4Step := &commonmodels.StepTask{
			Name:     build.ServiceName + "-perforce",
			JobName:  jobTask.Name,
			StepType: config.StepPerforce,
			Spec:     step.StepP4Spec{Repos: p4Repos},
		}

		jobTaskSpec.Steps = append(jobTaskSpec.Steps, p4Step)
		// init debug before step
		debugBeforeStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-debug_before",
			JobName:  jobTask.Name,
			StepType: config.StepDebugBefore,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)
		// init shell step
		scripts := []string{}
		dockerLoginCmd := `docker login -u "$DOCKER_REGISTRY_AK" -p "$DOCKER_REGISTRY_SK" "$DOCKER_REGISTRY_HOST" &> /dev/null`
		if jobTask.Infrastructure == setting.JobVMInfrastructure {
			scripts = append(scripts, strings.Split(replaceWrapLine(buildInfo.Scripts), "\n")...)
		} else {
			scripts = append([]string{dockerLoginCmd}, strings.Split(replaceWrapLine(buildInfo.Scripts), "\n")...)
			scripts = append(scripts, outputScript(outputs, jobTask.Infrastructure)...)
		}
		scriptStep := &commonmodels.StepTask{
			JobName: jobTask.Name,
		}
		if buildInfo.ScriptType == types.ScriptTypeShell || buildInfo.ScriptType == "" {
			scriptStep.Name = build.ServiceName + "-shell"
			scriptStep.StepType = config.StepShell
			scriptStep.Spec = &step.StepShellSpec{
				Scripts: scripts,
			}
		} else if buildInfo.ScriptType == types.ScriptTypeBatchFile {
			scriptStep.Name = build.ServiceName + "-batchfile"
			scriptStep.StepType = config.StepBatchFile
			scriptStep.Spec = &step.StepBatchFileSpec{
				Scripts: scripts,
			}
		} else if buildInfo.ScriptType == types.ScriptTypePowerShell {
			scriptStep.Name = build.ServiceName + "-powershell"
			scriptStep.StepType = config.StepPowerShell
			scriptStep.Spec = &step.StepPowerShellSpec{
				Scripts: scripts,
			}
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)
		// init debug after step
		debugAfterStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-debug_after",
			JobName:  jobTask.Name,
			StepType: config.StepDebugAfter,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)
		// init docker build step
		if buildInfo.PostBuild != nil && buildInfo.PostBuild.DockerBuild != nil {
			dockefileContent := ""
			if buildInfo.PostBuild.DockerBuild.TemplateID != "" {
				if dockerfileDetail, err := templ.GetDockerfileTemplateDetail(buildInfo.PostBuild.DockerBuild.TemplateID, logger); err == nil {
					dockefileContent = dockerfileDetail.Content
				}
			}

			dockerBuildStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-docker-build",
				JobName:  jobTask.Name,
				StepType: config.StepDockerBuild,
				Spec: step.StepDockerBuildSpec{
					Source:                buildInfo.PostBuild.DockerBuild.Source,
					WorkDir:               buildInfo.PostBuild.DockerBuild.WorkDir,
					DockerFile:            buildInfo.PostBuild.DockerBuild.DockerFile,
					ImageName:             image,
					ImageReleaseTag:       imageTag,
					BuildArgs:             buildInfo.PostBuild.DockerBuild.BuildArgs,
					DockerTemplateContent: dockefileContent,
					EnableBuildkit:        buildInfo.PostBuild.DockerBuild.EnableBuildkit,
					Platform:              buildInfo.PostBuild.DockerBuild.Platform,
					BuildKitImage:         config.BuildKitImage(),
					DockerRegistry: &step.DockerRegistry{
						DockerRegistryID: j.jobSpec.DockerRegistryID,
						Host:             registry.RegAddr,
						UserName:         registry.AccessKey,
						Password:         registry.SecretKey,
						Namespace:        registry.Namespace,
					},
					Repos: repos,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, dockerBuildStep)
		}

		// init object cache step
		if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
			cacheDir := "/workspace"
			if jobTaskSpec.Properties.CacheDirType == types.UserDefinedCacheDir {
				cacheDir = jobTaskSpec.Properties.CacheUserDir
			}
			tarArchiveStep := &commonmodels.StepTask{
				Name:     fmt.Sprintf("%s-%s", build.ServiceName, "tar-archive"),
				JobName:  jobTask.Name,
				StepType: config.StepTarArchive,
				Spec: step.StepTarArchiveSpec{
					FileName:     setting.BuildOSSCacheFileName,
					ResultDirs:   []string{"."},
					AbsResultDir: true,
					TarDir:       cacheDir,
					ChangeTarDir: true,
					S3DestDir:    getBuildJobCacheObjectPath(j.workflow.Name, build.ServiceName, build.ServiceModule),
					IgnoreErr:    true,
					S3Storage:    modelToS3StepSpec(cacheS3),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
		}

		// init archive step
		if buildInfo.PostBuild != nil && buildInfo.PostBuild.FileArchive != nil && buildInfo.PostBuild.FileArchive.FileLocation != "" {
			uploads := []*step.Upload{
				{
					IsFileArchive:       true,
					Name:                pkgFile,
					ServiceName:         build.ServiceName,
					ServiceModule:       build.ServiceModule,
					JobTaskName:         jobTask.Name,
					PackageFileLocation: buildInfo.PostBuild.FileArchive.FileLocation,
					FilePath:            path.Join(buildInfo.PostBuild.FileArchive.FileLocation, pkgFile),
					DestinationPath:     path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "archive"),
				},
			}
			archiveStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-pkgfile-archive",
				JobName:  jobTask.Name,
				StepType: config.StepArchive,
				Spec: step.StepArchiveSpec{
					UploadDetail: uploads,
					S3:           modelToS3StepSpec(defaultS3),
					Repos:        repos,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init object storage step
		if buildInfo.PostBuild != nil && buildInfo.PostBuild.ObjectStorageUpload != nil && buildInfo.PostBuild.ObjectStorageUpload.Enabled {
			modelS3, err := commonrepo.NewS3StorageColl().Find(buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID)
			if err != nil {
				return nil, fmt.Errorf("find object storage: %s failed, err: %v", buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID, err)
			}
			s3 := modelToS3StepSpec(modelS3)
			s3.Subfolder = ""
			uploads := []*step.Upload{}
			for _, detail := range buildInfo.PostBuild.ObjectStorageUpload.UploadDetail {
				uploads = append(uploads, &step.Upload{
					FilePath:        detail.FilePath,
					DestinationPath: detail.DestinationPath,
				})
			}
			archiveStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-object-storage",
				JobName:  jobTask.Name,
				StepType: config.StepArchive,
				Spec: step.StepArchiveSpec{
					UploadDetail:    uploads,
					ObjectStorageID: buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID,
					S3:              s3,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init post build shell step
		if buildInfo.PostBuild != nil && buildInfo.PostBuild.Scripts != "" {
			scripts := append([]string{dockerLoginCmd}, strings.Split(replaceWrapLine(buildInfo.PostBuild.Scripts), "\n")...)
			shellStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-post-shell",
				JobName:  jobTask.Name,
				StepType: config.StepShell,
				Spec: &step.StepShellSpec{
					Scripts: scripts,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, shellStep)
		}

		renderedTask, err := replaceServiceAndModulesForTask(jobTask, build.ServiceName, build.ServiceModule)
		if err != nil {
			return nil, fmt.Errorf("failed to render service variables, error: %v", err)
		}

		resp = append(resp, renderedTask)
	}

	return resp, nil
}

func (j BuildJobController) SetRepo(repo *types.Repository) error {
	for _, build := range j.jobSpec.ServiceAndBuilds {
		build.Repos = applyRepos(build.Repos, []*types.Repository{repo})
	}

	return nil
}

func (j BuildJobController) SetRepoCommitInfo() error {
	for _, build := range j.jobSpec.ServiceAndBuilds {
		if err := setRepoInfo(build.Repos); err != nil {
			return err
		}
	}

	return nil
}

func (j BuildJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	if getAggregatedVariables {
		var serviceAndModuleName, branchList, gitURLs []string
		for _, serviceAndBuild := range j.jobSpec.ServiceAndBuilds {
			serviceAndModuleName = append(serviceAndModuleName, serviceAndBuild.ServiceModule+"/"+serviceAndBuild.ServiceName)
			branch, gitURL := "", ""
			if len(serviceAndBuild.Repos) > 0 {
				branch = serviceAndBuild.Repos[0].Branch
				if serviceAndBuild.Repos[0].AuthType == types.SSHAuthType {
					gitURL = fmt.Sprintf("%s:%s/%s", serviceAndBuild.Repos[0].Address, serviceAndBuild.Repos[0].RepoOwner, serviceAndBuild.Repos[0].RepoName)
				} else {
					gitURL = fmt.Sprintf("%s/%s/%s", serviceAndBuild.Repos[0].Address, serviceAndBuild.Repos[0].RepoOwner, serviceAndBuild.Repos[0].RepoName)
				}
			}
			branchList = append(branchList, branch)
			gitURLs = append(gitURLs, gitURL)
		}
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "SERVICES"}, "."),
			Value:        strings.Join(serviceAndModuleName, ","),
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "BRANCHES"}, "."),
			Value:        strings.Join(branchList, ","),
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "IMAGES"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "GITURLS"}, "."),
			Value:        strings.Join(gitURLs, ","),
			Type:         "string",
			IsCredential: false,
		})
	}

	if getRuntimeVariables {
		outputKeys := sets.NewString()
		buildSvc := commonservice.NewBuildService()
		for _, build := range j.jobSpec.ServiceAndBuildsOptions {
			jobKey := strings.Join([]string{j.name, build.ServiceName, build.ServiceModule}, ".")
			buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
			if err != nil {
				return nil, err
			}
			for _, output := range ensureBuiltInBuildOutputs(buildInfo.Outputs) {
				if getServiceSpecificVariables {
					resp = append(resp, &commonmodels.KeyVal{
						Key:          strings.Join([]string{"job", jobKey, "output", output.Name}, "."),
						Value:        "",
						Type:         "string",
						IsCredential: false,
					})
				}
				outputKeys = outputKeys.Insert(output.Name)
			}
			// Add status variable for each service/module
			if getServiceSpecificVariables {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{"job", jobKey, "status"}, "."),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
		}
		if getPlaceHolderVariables {
			jobKey := strings.Join([]string{j.name, "<SERVICE>", "<MODULE>"}, ".")
			for _, key := range outputKeys.List() {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          strings.Join([]string{"job", jobKey, "output", key}, "."),
					Value:        "",
					Type:         "string",
					IsCredential: false,
				})
			}
			// Add placeholder status variable
			resp = append(resp, &commonmodels.KeyVal{
				Key:          strings.Join([]string{"job", jobKey, "status"}, "."),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})
		}
	}

	if getPlaceHolderVariables {
		jobKey := strings.Join([]string{"job", j.name, "<SERVICE>", "<MODULE>"}, ".")
		resp = append(resp, &commonmodels.KeyVal{
			Key:          fmt.Sprintf("%s.%s", jobKey, GITURLKEY),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          fmt.Sprintf("%s.%s", jobKey, BRANCHKEY),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          fmt.Sprintf("%s.%s", jobKey, COMMITIDKEY),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          fmt.Sprintf("%s.%s", jobKey, REPONAMEKEY),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          fmt.Sprintf("%s.%s", jobKey, PRE_MEGRE_BRANCHES_KEY),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

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
		for _, service := range j.jobSpec.ServiceAndBuildsOptions {
			for _, keyVal := range service.KeyVals {
				keySet = keySet.Insert(keyVal.Key)
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

	buildSvc := commonservice.NewBuildService()

	if getServiceSpecificVariables {
		target := j.jobSpec.ServiceAndBuildsOptions
		if useUserInputValue {
			target = j.jobSpec.ServiceAndBuilds
		}

		for _, service := range target {
			build, err := buildSvc.GetBuild(service.BuildName, service.ServiceName, service.ServiceModule)
			if err != nil {
				return nil, err
			}

			kvs := applyKeyVals(build.PreBuild.Envs.ToRuntimeList(), service.KeyVals, true)

			jobKey := strings.Join([]string{"job", j.name, service.ServiceName, service.ServiceModule}, ".")
			for _, keyVal := range kvs {
				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, keyVal.Key),
					Value:        keyVal.GetValue(),
					Type:         "string",
					IsCredential: false,
				})
			}

			repos := applyRepos(build.Repos, service.Repos)

			for _, repo := range repos {
				repoInfo, err := systemconfig.New().GetCodeHost(repo.CodehostID)
				if err != nil {
					return nil, err
				}

				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, BRANCHKEY),
					Value:        repo.Branch,
					Type:         "string",
					IsCredential: false,
				})

				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, COMMITIDKEY),
					Value:        repo.CommitID,
					Type:         "string",
					IsCredential: false,
				})

				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, REPONAMEKEY),
					Value:        repo.RepoName,
					Type:         "string",
					IsCredential: false,
				})

				if len(repo.MergeBranches) > 0 {
					resp = append(resp, &commonmodels.KeyVal{
						Key:          fmt.Sprintf("%s.%s", jobKey, PRE_MEGRE_BRANCHES_KEY),
						Value:        repo.GetPreMergeBranches(),
						Type:         "string",
						IsCredential: false,
					})
				} else {
					resp = append(resp, &commonmodels.KeyVal{
						Key:          fmt.Sprintf("%s.%s", jobKey, PRE_MEGRE_BRANCHES_KEY),
						Value:        "",
						Type:         "string",
						IsCredential: false,
					})
				}

				var gitURL string
				if repo.AuthType == types.SSHAuthType {
					gitURL = fmt.Sprintf("%s:%s/%s", repoInfo.Address, repo.RepoOwner, repo.RepoName)
				} else {
					gitURL = fmt.Sprintf("%s/%s/%s", repoInfo.Address, repo.RepoOwner, repo.RepoName)
				}

				resp = append(resp, &commonmodels.KeyVal{
					Key:          fmt.Sprintf("%s.%s", jobKey, GITURLKEY),
					Value:        gitURL,
					Type:         "string",
					IsCredential: false,
				})
				break
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

func (j BuildJobController) GetUsedRepos() ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	buildSvc := commonservice.NewBuildService()
	for _, build := range j.jobSpec.ServiceAndBuildsOptions {
		buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				resp = append(resp, applyRepos(buildInfo.Repos, build.Repos)...)
				break
			}
		}
	}
	return resp, nil
}

func (j BuildJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	// In build job, service name and service module must be provided
	if option.ServiceName == "" || option.ServiceModule == "" {
		return nil, fmt.Errorf("service name and service module are required for build job")
	}

	// Find the correct service/module from options
	var targetBuild *commonmodels.ServiceAndBuild
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return nil, fmt.Errorf("failed to find job: %s in workflow %s's latest config, error: %s", j.name, j.workflow.Name, err)
	}
	latestJobSpec := new(commonmodels.ZadigBuildJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestJobSpec); err != nil {
		return nil, fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	for _, build := range latestJobSpec.ServiceAndBuildsOptions {
		if build.ServiceName == option.ServiceName && build.ServiceModule == option.ServiceModule {
			targetBuild = build
			break
		}
	}

	if targetBuild == nil {
		return nil, fmt.Errorf("service %s/%s not found in build job options", option.ServiceName, option.ServiceModule)
	}

	// Find the KeyVal with the given key
	for _, kv := range targetBuild.KeyVals {
		if kv.Key == key {
			resp, err := renderScriptedVariableOptions(option.ServiceName, option.ServiceModule, kv.Script, kv.CallFunction, option.Values)
			if err != nil {
				err = fmt.Errorf("Failed to render kv for key: %s, error: %s", key, err)
				return nil, err
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("key: %s not found in service %s/%s of job: %s", key, option.ServiceName, option.ServiceModule, j.name)
}

func (j BuildJobController) IsServiceTypeJob() bool {
	return true
}

func (j BuildJobController) getReferredJobTargets(jobName string) ([]*commonmodels.ServiceAndBuild, error) {
	servicetargets := []*commonmodels.ServiceAndBuild{}
	originTargetMap := make(map[string]*commonmodels.ServiceAndBuild)

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return nil, fmt.Errorf("get services map error: %v", err)
	}

	for _, build := range j.jobSpec.ServiceAndBuilds {
		target := &commonmodels.ServiceAndBuild{
			ServiceName:      build.ServiceName,
			ServiceModule:    build.ServiceModule,
			BuildName:        build.BuildName,
			ImageName:        build.ImageName,
			KeyVals:          build.KeyVals,
			Repos:            build.Repos,
			ShareStorageInfo: build.ShareStorageInfo,
		}

		service := servicesMap[build.ServiceName]
		if service == nil {
			return nil, fmt.Errorf("service %s not found", build.ServiceName)
		}
		for _, container := range service.Containers {
			if container.Name == build.ServiceModule {
				target.ImageName = container.ImageName
				break
			}
		}

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
					return servicetargets, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					target := &commonmodels.ServiceAndBuild{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
					}
					if originService, ok := originTargetMap[build.GetKey()]; ok {
						target.KeyVals = originService.KeyVals
						target.Repos = originService.Repos
						target.BuildName = originService.BuildName
						target.ImageName = originService.ImageName
						target.ShareStorageInfo = originService.ShareStorageInfo
					} else {
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
						continue
					}

					servicetargets = append(servicetargets, target)
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return servicetargets, err
				}
				for _, distribute := range distributeSpec.Targets {
					target := &commonmodels.ServiceAndBuild{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
					}
					if originService, ok := originTargetMap[target.GetKey()]; ok {
						target.KeyVals = originService.KeyVals
						target.Repos = originService.Repos
						target.BuildName = originService.BuildName
						target.ImageName = originService.ImageName
						target.ShareStorageInfo = originService.ShareStorageInfo
					} else {
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
						continue
					}
					servicetargets = append(servicetargets, target)
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
						target := &commonmodels.ServiceAndBuild{
							ServiceName:   svc.ServiceName,
							ServiceModule: module.ServiceModule,
						}
						if originService, ok := originTargetMap[target.GetKey()]; ok {
							target.KeyVals = originService.KeyVals
							target.Repos = originService.Repos
							target.BuildName = originService.BuildName
							target.ImageName = originService.ImageName
							target.ShareStorageInfo = originService.ShareStorageInfo
						} else {
							log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
							continue
						}
						servicetargets = append(servicetargets, target)
					}
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigTesting {
				testingSpec := &commonmodels.ZadigTestingJobSpec{}
				if err := commonmodels.IToi(job.Spec, testingSpec); err != nil {
					return servicetargets, err
				}
				for _, svc := range testingSpec.ServiceAndTests {
					target := &commonmodels.ServiceAndBuild{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					}
					if originService, ok := originTargetMap[target.GetKey()]; ok {
						target.KeyVals = originService.KeyVals
						target.Repos = originService.Repos
						target.BuildName = originService.BuildName
						target.ImageName = originService.ImageName
						target.ShareStorageInfo = originService.ShareStorageInfo
					} else {
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
						continue
					}
					servicetargets = append(servicetargets, target)
				}
				return servicetargets, nil
			}
			if job.JobType == config.JobZadigScanning {
				scanningSpec := &commonmodels.ZadigScanningJobSpec{}
				if err := commonmodels.IToi(job.Spec, scanningSpec); err != nil {
					return servicetargets, err
				}
				for _, svc := range scanningSpec.ServiceAndScannings {
					target := &commonmodels.ServiceAndBuild{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					}
					if originService, ok := originTargetMap[target.GetKey()]; ok {
						target.KeyVals = originService.KeyVals
						target.Repos = originService.Repos
						target.BuildName = originService.BuildName
						target.ImageName = originService.ImageName
						target.ShareStorageInfo = originService.ShareStorageInfo
					} else {
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
						continue
					}
					servicetargets = append(servicetargets, target)
				}
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
					target := &commonmodels.ServiceAndBuild{
						ServiceName:   svc.ServiceName,
						ServiceModule: svc.ServiceModule,
					}
					if originService, ok := originTargetMap[target.GetKey()]; ok {
						target.KeyVals = originService.KeyVals
						target.Repos = originService.Repos
						target.BuildName = originService.BuildName
						target.ImageName = originService.ImageName
						target.ShareStorageInfo = originService.ShareStorageInfo
					} else {
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.name)
						continue
					}
					servicetargets = append(servicetargets, target)
				}
				return servicetargets, nil
			}
		}
	}
	return nil, fmt.Errorf("BuilJob: refered job %s not found", jobName)
}

func getBuildJobCacheObjectPath(workflowName, serviceName, serviceModule string) string {
	return fmt.Sprintf("%s/cache/%s/%s", workflowName, serviceName, serviceModule)
}

func getBuildJobVariables(build *commonmodels.ServiceAndBuild, taskID int64, project, workflowName, workflowDisplayName, image, pkgFile, infrastructure string, registry *commonmodels.RegistryNamespace, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, prepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)

	// repo envs
	ret = append(ret, getReposVariables(build.Repos)...)

	// build specific envs
	registryHost := strings.TrimPrefix(registry.RegAddr, "http://")
	registryHost = strings.TrimPrefix(registryHost, "https://")
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_HOST", Value: registryHost, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_NAMESPACE", Value: registry.Namespace, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_AK", Value: registry.AccessKey, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_SK", Value: registry.SecretKey, IsCredential: true})

	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: build.ServiceModule, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "IMAGE", Value: image, IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), project, workflowName, taskID, url.QueryEscape(workflowDisplayName))
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: pkgFile, IsCredential: false})

	// TODO: remove it
	ret = append(ret, &commonmodels.KeyVal{Key: "GIT_SSL_NO_VERIFY", Value: "true", IsCredential: false})
	return ret
}

func ensureBuiltInBuildOutputs(outputs []*commonmodels.Output) []*commonmodels.Output {
	keyMap := map[string]struct{}{}
	for _, output := range outputs {
		keyMap[output.Name] = struct{}{}
	}
	if _, ok := keyMap[IMAGEKEY]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: IMAGEKEY,
		})
	}
	if _, ok := keyMap[IMAGETAGKEY]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: IMAGETAGKEY,
		})
	}
	if _, ok := keyMap[PKGFILEKEY]; !ok {
		outputs = append(outputs, &commonmodels.Output{
			Name: PKGFILEKEY,
		})
	}
	return outputs
}
