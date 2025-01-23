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
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

const (
	IMAGEKEY    = "IMAGE"
	IMAGETAGKEY = "imageTag"
	PKGFILEKEY  = "PKG_FILE"
	BRANCHKEY   = "BRANCH"
	REPONAMEKEY = "REPONAME"
	GITURLKEY   = "GITURL"
	COMMITIDKEY = "COMMITID"
)

type BuildJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigBuildJobSpec
}

func (j *BuildJob) Instantiate() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

// SetPreset will clear the selected field (ServiceAndBuilds
func (j *BuildJob) SetPreset() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return fmt.Errorf("get services map error: %v", err)
	}

	buildSvc := commonservice.NewBuildService()
	newBuilds := make([]*commonmodels.ServiceAndBuild, 0)
	for _, build := range j.spec.ServiceAndBuilds {
		buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				build.Repos = mergeRepos(buildInfo.Repos, build.Repos)
				build.KeyVals = renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs)
				break
			}
		}

		build.ImageName = build.ServiceModule
		service, ok := servicesMap[build.ServiceName]
		if !ok {
			log.Errorf("service %s not found", build.ServiceName)
			continue
		}

		for _, container := range service.Containers {
			if container.Name == build.ServiceModule {
				build.ImageName = container.ImageName
				break
			}
		}

		newBuilds = append(newBuilds, build)
	}
	j.spec.ServiceAndBuilds = newBuilds

	defaultServiceAndBuildMap := make(map[string]*commonmodels.ServiceAndBuild)
	for _, svc := range j.spec.DefaultServiceAndBuilds {
		defaultServiceAndBuildMap[svc.GetKey()] = svc
	}

	newDefaultBuilds := make([]*commonmodels.ServiceAndBuild, 0)
	for _, build := range j.spec.ServiceAndBuilds {
		if _, ok := defaultServiceAndBuildMap[build.GetKey()]; ok {
			newDefaultBuilds = append(newDefaultBuilds, build)
		}
	}
	j.spec.DefaultServiceAndBuilds = newDefaultBuilds

	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) ClearSelectionField() error {
	return nil
}

func (j *BuildJob) SetOptions(approvalTicket *commonmodels.ApprovalTicket) error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	originalWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
	}

	originalSpec := new(commonmodels.ZadigBuildJobSpec)
	found := false
	for _, stage := range originalWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, originalSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return fmt.Errorf("get services map error: %v", err)
	}

	var allowedServices []*commonmodels.ServiceWithModule
	if approvalTicket != nil {
		allowedServices = approvalTicket.Services
	}

	buildSvc := commonservice.NewBuildService()
	newBuilds := make([]*commonmodels.ServiceAndBuild, 0)
	for _, build := range originalSpec.ServiceAndBuilds {
		if !isAllowedService(build.ServiceName, build.ServiceModule, allowedServices) {
			continue
		}

		buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				build.Repos = mergeRepos(buildInfo.Repos, build.Repos)
				build.KeyVals = renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs)
				break
			}
		}

		build.ImageName = build.ServiceModule
		service, ok := servicesMap[build.ServiceName]
		if !ok {
			log.Errorf("service %s not found", build.ServiceName)
			continue
		}

		for _, container := range service.Containers {
			if container.Name == build.ServiceModule {
				build.ImageName = container.ImageName
				break
			}
		}

		newBuilds = append(newBuilds, build)
	}

	j.spec.ServiceAndBuildsOptions = newBuilds
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) ClearOptions() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.spec.ServiceAndBuildsOptions = nil
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) GetRepos() ([]*types.Repository, error) {
	resp := []*types.Repository{}
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	buildSvc := commonservice.NewBuildService()
	for _, build := range j.spec.ServiceAndBuilds {
		buildInfo, err := buildSvc.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				resp = append(resp, mergeRepos(buildInfo.Repos, build.Repos)...)
				break
			}
		}
	}
	return resp, nil
}

func (j *BuildJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigBuildJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigBuildJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		newBuilds := []*commonmodels.ServiceAndBuild{}
		for _, build := range j.spec.ServiceAndBuilds {
			for _, argsBuild := range argsSpec.ServiceAndBuilds {
				if build.BuildName == argsBuild.BuildName && build.ServiceName == argsBuild.ServiceName && build.ServiceModule == argsBuild.ServiceModule {
					build.Repos = mergeRepos(build.Repos, argsBuild.Repos)
					build.KeyVals = renderKeyVals(argsBuild.KeyVals, build.KeyVals)
					newBuilds = append(newBuilds, build)
					break
				}
			}
		}
		j.spec.ServiceAndBuilds = newBuilds

		newDefaultBuilds := []*commonmodels.ServiceAndBuild{}
		for _, build := range j.spec.ServiceAndBuilds {
			for _, argsBuild := range argsSpec.DefaultServiceAndBuilds {
				if build.BuildName == argsBuild.BuildName && build.ServiceName == argsBuild.ServiceName && build.ServiceModule == argsBuild.ServiceModule {
					build.Repos = mergeRepos(build.Repos, argsBuild.Repos)
					build.KeyVals = renderKeyVals(argsBuild.KeyVals, build.KeyVals)
					newDefaultBuilds = append(newDefaultBuilds, build)
					break
				}
			}
		}
		j.spec.DefaultServiceAndBuilds = newDefaultBuilds

		j.job.Spec = j.spec
	}
	return nil
}

func (j *BuildJob) UpdateWithLatestSetting() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	latestWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
	}

	latestSpec := new(commonmodels.ZadigBuildJobSpec)
	found := false
	for _, stage := range latestWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, latestSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}

	// save all the user-defined args into a map
	userConfiguredService := make(map[string]*commonmodels.ServiceAndBuild)

	for _, service := range j.spec.ServiceAndBuilds {
		key := fmt.Sprintf("%s++%s", service.ServiceName, service.ServiceModule)
		userConfiguredService[key] = service
	}

	buildSvc := commonservice.NewBuildService()
	mergedServiceAndBuilds := make([]*commonmodels.ServiceAndBuild, 0)
	for _, buildInfo := range latestSpec.ServiceAndBuilds {
		key := fmt.Sprintf("%s++%s", buildInfo.ServiceName, buildInfo.ServiceModule)
		// if a service is selected (in the map above) and is in the latest build job config, add it to the list.
		// user defined kv and repo should be merged into the newly created list.
		if userDefinedArgs, ok := userConfiguredService[key]; ok {
			latestBuild, err := buildSvc.GetBuild(buildInfo.BuildName, buildInfo.ServiceName, buildInfo.ServiceModule)
			if err != nil {
				log.Errorf("find build: %s error: %v", buildInfo.BuildName, err)
				continue
			}

			for _, target := range latestBuild.Targets {
				if target.ServiceName == buildInfo.ServiceName && target.ServiceModule == buildInfo.ServiceModule {
					buildInfo.Repos = mergeRepos(latestBuild.Repos, buildInfo.Repos)
					buildInfo.KeyVals = renderKeyVals(buildInfo.KeyVals, latestBuild.PreBuild.Envs)
					break
				}
			}

			newBuildInfo := &commonmodels.ServiceAndBuild{
				ServiceName:      buildInfo.ServiceName,
				ServiceModule:    buildInfo.ServiceModule,
				BuildName:        buildInfo.BuildName,
				Image:            buildInfo.Image,
				Package:          buildInfo.Package,
				ImageName:        buildInfo.ImageName,
				KeyVals:          renderKeyVals(userDefinedArgs.KeyVals, buildInfo.KeyVals),
				Repos:            mergeRepos(buildInfo.Repos, userDefinedArgs.Repos),
				ShareStorageInfo: buildInfo.ShareStorageInfo,
			}

			mergedServiceAndBuilds = append(mergedServiceAndBuilds, newBuildInfo)
		} else {
			continue
		}
	}

	j.spec.DockerRegistryID = latestSpec.DockerRegistryID
	j.spec.ServiceAndBuilds = mergedServiceAndBuilds
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) MergeWebhookRepo(webhookRepo *types.Repository) error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	for _, build := range j.spec.ServiceAndBuilds {
		build.Repos = mergeRepos(build.Repos, []*types.Repository{webhookRepo})
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	registry, err := commonservice.FindRegistryById(j.spec.DockerRegistryID, true, logger)
	if err != nil {
		return resp, fmt.Errorf("find docker registry: %s error: %v", j.spec.DockerRegistryID, err)
	}
	defaultS3, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return resp, fmt.Errorf("find default s3 storage error: %v", err)
	}

	if j.spec.Source == config.SourceFromJob {
		referredJob := getOriginJobName(j.workflow, j.spec.JobName)
		targets, err := j.getOriginReferedJobTargets(referredJob)
		if err != nil {
			return resp, fmt.Errorf("build job %s, get origin refered job: %s targets failed, err: %v", j.spec.JobName, referredJob, err)
		}
		j.spec.ServiceAndBuilds = targets
	}

	buildSvc := commonservice.NewBuildService()
	for jobSubTaskID, build := range j.spec.ServiceAndBuilds {
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
			return resp, fmt.Errorf("find build: %s error: %v", build.BuildName, err)
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(buildInfo.PreBuild.ImageID)
		if err != nil {
			return resp, fmt.Errorf("find base image: %s error: %v", buildInfo.PreBuild.ImageID, err)
		}
		registries, err := commonservice.ListRegistryNamespaces("", true, logger)
		if err != nil {
			return resp, err
		}
		outputs := ensureBuildInOutputs(buildInfo.Outputs)
		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			JobInfo: map[string]string{
				"service_name":   build.ServiceName,
				"service_module": build.ServiceModule,
				JobNameKey:       j.job.Name,
			},
			Key:            genJobKey(j.job.Name, build.ServiceName, build.ServiceModule),
			Name:           GenJobName(j.workflow, j.job.Name, jobSubTaskID),
			DisplayName:    genJobDisplayName(j.job.Name, build.ServiceName, build.ServiceModule),
			OriginName:     j.job.Name,
			JobType:        string(config.JobZadigBuild),
			Spec:           jobTaskSpec,
			Timeout:        int64(buildInfo.Timeout),
			Outputs:        outputs,
			Infrastructure: buildInfo.Infrastructure,
			VMLabels:       buildInfo.VMLabels,
			ErrorPolicy:    j.job.ErrorPolicy,
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:             int64(buildInfo.Timeout),
			ResourceRequest:     buildInfo.PreBuild.ResReq,
			ResReqSpec:          buildInfo.PreBuild.ResReqSpec,
			CustomEnvs:          renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs),
			ClusterID:           buildInfo.PreBuild.ClusterID,
			StrategyID:          buildInfo.PreBuild.StrategyID,
			BuildOS:             basicImage.Value,
			ImageFrom:           buildInfo.PreBuild.ImageFrom,
			Registries:          registries,
			ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, build.ShareStorageInfo, j.workflow.Name, taskID),
			CustomLabels:        buildInfo.PreBuild.CustomLabels,
			CustomAnnotations:   buildInfo.PreBuild.CustomAnnotations,
		}

		paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
		envs := mergeKeyVals(jobTaskSpec.Properties.CustomEnvs, paramEnvs)

		jobTaskSpec.Properties.Envs = append(envs, getBuildJobVariables(build, taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, image, pkgFile, jobTask.Infrastructure, registry, logger)...)
		jobTaskSpec.Properties.UseHostDockerDaemon = buildInfo.PreBuild.UseHostDockerDaemon

		cacheS3 := &commonmodels.S3Storage{}
		if jobTask.Infrastructure == setting.JobVMInfrastructure {
			jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
		} else {
			clusterInfo, err := commonrepo.NewK8SClusterColl().Get(buildInfo.PreBuild.ClusterID)
			if err != nil {
				return resp, fmt.Errorf("find cluster: %s error: %v", buildInfo.PreBuild.ClusterID, err)
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
						return resp, fmt.Errorf("find cache s3 storage: %s error: %v", jobTaskSpec.Properties.Cache.ObjectProperties.ID, err)
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
					S3:         modelS3toS3(cacheS3),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArchiveStep)
		}

		codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
		if err != nil {
			return resp, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
		}

		// init git clone step
		repos := renderRepos(build.Repos, buildInfo.Repos, jobTaskSpec.Properties.Envs)
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
					DockerRegistry: &step.DockerRegistry{
						DockerRegistryID: j.spec.DockerRegistryID,
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
					S3Storage:    modelS3toS3(cacheS3),
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
					S3:           modelS3toS3(defaultS3),
					Repos:        repos,
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init object storage step
		if buildInfo.PostBuild != nil && buildInfo.PostBuild.ObjectStorageUpload != nil && buildInfo.PostBuild.ObjectStorageUpload.Enabled {
			modelS3, err := commonrepo.NewS3StorageColl().Find(buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID)
			if err != nil {
				return resp, fmt.Errorf("find object storage: %s failed, err: %v", buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID, err)
			}
			s3 := modelS3toS3(modelS3)
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
		resp = append(resp, jobTask)
	}
	j.job.Spec = j.spec
	return resp, nil
}

func renderKeyVals(input, origin []*commonmodels.KeyVal) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)

	for _, originKV := range origin {
		item := &commonmodels.KeyVal{
			Key:               originKV.Key,
			Value:             originKV.Value,
			Type:              originKV.Type,
			IsCredential:      originKV.IsCredential,
			ChoiceOption:      originKV.ChoiceOption,
			Description:       originKV.Description,
			FunctionReference: originKV.FunctionReference,
			CallFunction:      originKV.CallFunction,
			Script:            originKV.Script,
		}
		for _, inputKV := range input {
			if originKV.Key == inputKV.Key {
				if originKV.Type == commonmodels.MultiSelectType {
					item.ChoiceValue = inputKV.ChoiceValue
					if !strings.HasPrefix(inputKV.Value, "{{.") {
						item.Value = strings.Join(item.ChoiceValue, ",")
					} else {
						item.Value = inputKV.Value
					}
				} else {
					// always use origin credential config.
					item.Value = inputKV.Value
				}

			}
		}
		resp = append(resp, item)
	}
	return resp
}

// mergeKeyVals merges kv pairs from source1 and source2, and if the key collides, use source 1's value
func mergeKeyVals(source1, source2 []*commonmodels.KeyVal) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)
	existingKVMap := make(map[string]*commonmodels.KeyVal)

	for _, src1KV := range source1 {
		if _, ok := existingKVMap[src1KV.Key]; ok {
			continue
		}
		item := &commonmodels.KeyVal{
			Key:               src1KV.Key,
			Value:             src1KV.Value,
			Type:              src1KV.Type,
			IsCredential:      src1KV.IsCredential,
			ChoiceValue:       src1KV.ChoiceValue,
			ChoiceOption:      src1KV.ChoiceOption,
			Description:       src1KV.Description,
			FunctionReference: src1KV.FunctionReference,
			CallFunction:      src1KV.CallFunction,
			Script:            src1KV.Script,
		}
		existingKVMap[src1KV.Key] = src1KV
		resp = append(resp, item)
	}

	for _, src2KV := range source2 {
		if _, ok := existingKVMap[src2KV.Key]; ok {
			continue
		}

		item := &commonmodels.KeyVal{
			Key:               src2KV.Key,
			Value:             src2KV.Value,
			Type:              src2KV.Type,
			IsCredential:      src2KV.IsCredential,
			ChoiceValue:       src2KV.ChoiceValue,
			ChoiceOption:      src2KV.ChoiceOption,
			Description:       src2KV.Description,
			FunctionReference: src2KV.FunctionReference,
			CallFunction:      src2KV.CallFunction,
			Script:            src2KV.Script,
		}
		existingKVMap[src2KV.Key] = src2KV
		resp = append(resp, item)
	}
	return resp
}

// generateKeyValsFromWorkflowParam generates kv from workflow parameters, ditching all parameters of repo type
func generateKeyValsFromWorkflowParam(params []*commonmodels.Param) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)

	for _, param := range params {
		if param.ParamsType == "repo" {
			continue
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:               param.Name,
			Value:             param.Value,
			Type:              commonmodels.ParameterSettingType(param.ParamsType),
			RegistryID:        "",
			ChoiceOption:      param.ChoiceOption,
			ChoiceValue:       param.ChoiceValue,
			Script:            "",
			CallFunction:      "",
			FunctionReference: nil,
			IsCredential:      param.IsCredential,
			Description:       param.Description,
		})
	}

	return resp
}

func renderRepos(input, origin []*types.Repository, kvs []*commonmodels.KeyVal) []*types.Repository {
	resp := make([]*types.Repository, 0)
	for i, originRepo := range origin {
		resp = append(resp, originRepo)
		for _, inputRepo := range input {
			if originRepo.RepoName == inputRepo.RepoName && originRepo.RepoOwner == inputRepo.RepoOwner {
				inputRepo.CheckoutPath = commonutil.RenderEnv(inputRepo.CheckoutPath, kvs)
				if inputRepo.RemoteName == "" {
					inputRepo.RemoteName = "origin"
				}
				resp[i] = inputRepo
			}
		}
	}
	return resp
}

// splitReposByType split the repository by types. currently it will return non-perforce repos and perforce repos
func splitReposByType(repos []*types.Repository) (gitRepos, p4Repos []*types.Repository) {
	gitRepos = make([]*types.Repository, 0)
	p4Repos = make([]*types.Repository, 0)

	for _, repo := range repos {
		if repo.Source == types.ProviderPerforce {
			p4Repos = append(p4Repos, repo)
		} else {
			gitRepos = append(gitRepos, repo)
		}
	}

	return
}

func replaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}

func getBuildJobVariables(build *commonmodels.ServiceAndBuild, taskID int64, project, workflowName, workflowDisplayName, image, pkgFile, infrastructure string, registry *commonmodels.RegistryNamespace, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, PrepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)

	// repo envs
	ret = append(ret, getReposVariables(build.Repos)...)
	// build specific envs
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_HOST", Value: registry.RegAddr, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_AK", Value: registry.AccessKey, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_SK", Value: registry.SecretKey, IsCredential: true})

	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: build.ServiceModule, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "IMAGE", Value: image, IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), project, workflowName, taskID, url.QueryEscape(workflowDisplayName))
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: pkgFile, IsCredential: false})
	return ret
}

func modelS3toS3(modelS3 *commonmodels.S3Storage) *step.S3 {
	resp := &step.S3{
		Ak:        modelS3.Ak,
		Sk:        modelS3.Sk,
		Endpoint:  modelS3.Endpoint,
		Bucket:    modelS3.Bucket,
		Subfolder: modelS3.Subfolder,
		Insecure:  modelS3.Insecure,
		Provider:  modelS3.Provider,
		Region:    modelS3.Region,
		Protocol:  "https",
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}

func mergeRepos(templateRepos []*types.Repository, customRepos []*types.Repository) []*types.Repository {
	customRepoMap := make(map[string]*types.Repository)
	for _, repo := range customRepos {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		customRepoMap[repo.GetKey()] = repo
	}
	for _, repo := range templateRepos {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		// user can only set default branch in custom workflow.
		if cv, ok := customRepoMap[repo.GetKey()]; ok {
			repo.Branch = cv.Branch
			repo.Tag = cv.Tag
			repo.PR = cv.PR
			repo.PRs = cv.PRs
			repo.FilterRegexp = cv.FilterRegexp
		}
	}
	return templateRepos
}

func (j *BuildJob) LintJob() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}

	return nil
}

func (j *BuildJob) GetOutPuts(log *zap.SugaredLogger) []string {
	resp := []string{}
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return resp
	}

	outputKeys := sets.NewString()
	for _, build := range j.spec.ServiceAndBuilds {
		jobKey := strings.Join([]string{j.job.Name, build.ServiceName, build.ServiceModule}, ".")
		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
		if err != nil {
			log.Errorf("found build %s failed, err: %s", build.BuildName, err)
			continue
		}
		if buildInfo.TemplateID == "" {
			resp = append(resp, getOutputKey(jobKey, ensureBuildInOutputs(buildInfo.Outputs))...)
			for _, output := range ensureBuildInOutputs(buildInfo.Outputs) {
				outputKeys = outputKeys.Insert(output.Name)
			}
			continue
		}

		buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: buildInfo.TemplateID})
		if err != nil {
			log.Errorf("found build template %s failed, err: %s", buildInfo.TemplateID, err)
			continue
		}
		resp = append(resp, getOutputKey(jobKey, ensureBuildInOutputs(buildTemplate.Outputs))...)
		for _, output := range ensureBuildInOutputs(buildTemplate.Outputs) {
			outputKeys = outputKeys.Insert(output.Name)
		}
	}

	outputs := []*commonmodels.Output{}
	for _, outputKey := range outputKeys.List() {
		outputs = append(outputs, &commonmodels.Output{Name: outputKey})
	}
	resp = append(resp, getOutputKey(j.job.Name+".<SERVICE>.<MODULE>", outputs)...)
	resp = append(resp, "{{.job."+j.job.Name+".<SERVICE>.<MODULE>."+GITURLKEY+"}}")
	resp = append(resp, "{{.job."+j.job.Name+".<SERVICE>.<MODULE>."+BRANCHKEY+"}}")
	resp = append(resp, "{{.job."+j.job.Name+".<SERVICE>.<MODULE>."+COMMITIDKEY+"}}")
	return resp
}

func (j *BuildJob) GetRenderVariables(ctx *internalhandler.Context, jobName, serviceName, moduleName string, getAvaiableVars bool) ([]*commonmodels.KeyVal, error) {
	keyValMap := NewKeyValMap()
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		err = fmt.Errorf("failed to convert freestyle job spec error: %v", err)
		ctx.Logger.Error(err)
		return nil, err
	}

	for _, service := range j.spec.ServiceAndBuilds {
		if getAvaiableVars {
			branchKeyVal := &commonmodels.KeyVal{
				Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", BRANCHKEY, getAvaiableVars),
				Value: "",
			}
			commitidKeyVal := &commonmodels.KeyVal{
				Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", COMMITIDKEY, getAvaiableVars),
				Value: "",
			}
			repoNameKeyVal := &commonmodels.KeyVal{
				Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", REPONAMEKEY, getAvaiableVars),
				Value: "",
			}
			keyValMap.Insert(branchKeyVal, commitidKeyVal, repoNameKeyVal)

			for _, keyVal := range service.KeyVals {
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", keyVal.Key, getAvaiableVars),
					Value: keyVal.Value,
				})
			}
		} else {
			for _, keyVal := range service.KeyVals {
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, keyVal.Key, getAvaiableVars),
					Value: keyVal.Value,
				})
			}

			for _, repo := range service.Repos {
				// only use the first repo
				branchKeyVal := &commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, BRANCHKEY, getAvaiableVars),
					Value: repo.Branch,
				}
				commitidKeyVal := &commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, COMMITIDKEY, getAvaiableVars),
					Value: repo.CommitID,
				}
				repoNameKeyVal := &commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, REPONAMEKEY, getAvaiableVars),
					Value: repo.RepoName,
				}
				keyValMap.Insert(branchKeyVal, commitidKeyVal, repoNameKeyVal)

				break
			}
		}

		if jobName == j.job.Name {
			if getAvaiableVars {
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", "SERVICE_NAME", getAvaiableVars),
					Value: service.ServiceName,
				})
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, "<SERVICE>", "<MODULE>", "SERVICE_MODULE", getAvaiableVars),
					Value: service.ServiceModule,
				})
			} else {
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, "SERVICE_NAME", getAvaiableVars),
					Value: service.ServiceName,
				})
				keyValMap.Insert(&commonmodels.KeyVal{
					Key:   getJobVariableKey(j.job.Name, jobName, service.ServiceName, service.ServiceModule, "SERVICE_MODULE", getAvaiableVars),
					Value: service.ServiceModule,
				})
			}
		}
	}
	return keyValMap.List(), nil
}

func ensureBuildInOutputs(outputs []*commonmodels.Output) []*commonmodels.Output {
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

func getBuildJobCacheObjectPath(workflowName, serviceName, serviceModule string) string {
	return fmt.Sprintf("%s/cache/%s/%s", workflowName, serviceName, serviceModule)
}

func (j *BuildJob) getOriginReferedJobTargets(jobName string) ([]*commonmodels.ServiceAndBuild, error) {
	servicetargets := []*commonmodels.ServiceAndBuild{}
	originTargetMap := make(map[string]*commonmodels.ServiceAndBuild)

	servicesMap, err := repository.GetMaxRevisionsServicesMap(j.workflow.Project, false)
	if err != nil {
		return nil, fmt.Errorf("get services map error: %v", err)
	}

	for _, build := range j.spec.ServiceAndBuilds {
		target := &commonmodels.ServiceAndBuild{
			ServiceName:   build.ServiceName,
			ServiceModule: build.ServiceModule,
			BuildName:     build.BuildName,
			ImageName:     build.ImageName,
			KeyVals:       build.KeyVals,
			Repos:         build.Repos,
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
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
							log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
				for _, svc := range testingSpec.TargetServices {
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
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
				for _, svc := range scanningSpec.TargetServices {
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
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
						log.Warnf("service %s not found in %s job's config ", target.GetKey(), j.job.Name)
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
