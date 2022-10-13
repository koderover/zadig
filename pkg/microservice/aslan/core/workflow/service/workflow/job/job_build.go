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
	"os"
	"path"
	"strings"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	templ "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
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

func (j *BuildJob) SetPreset() error {
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	for _, build := range j.spec.ServiceAndBuilds {
		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		if err := fillBuildDetail(buildInfo, build.ServiceName, build.ServiceModule); err != nil {
			log.Errorf("fill build: %s detail error: %v", build.BuildName, err)
			continue
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				build.Repos = mergeRepos(buildInfo.Repos, build.Repos)
				build.KeyVals = renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs)
				break
			}
		}
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) GetRepos() ([]*types.Repository, error) {
	resp := []*types.Repository{}
	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	for _, build := range j.spec.ServiceAndBuilds {
		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
		if err != nil {
			log.Errorf("find build: %s error: %v", build.BuildName, err)
			continue
		}
		if err := fillBuildDetail(buildInfo, build.ServiceName, build.ServiceModule); err != nil {
			log.Errorf("fill build: %s detail error: %v", build.BuildName, err)
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
		j.spec.DockerRegistryID = argsSpec.DockerRegistryID
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
		j.job.Spec = j.spec
	}
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

	registry, _, err := commonservice.FindRegistryById(j.spec.DockerRegistryID, true, logger)
	if err != nil {
		return resp, err
	}
	defaultS3, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return resp, err
	}

	for _, build := range j.spec.ServiceAndBuilds {
		imageTag := commonservice.ReleaseCandidate(build.Repos, taskID, j.workflow.Project, build.ServiceModule, "", build.ServiceModule, "image")

		if len(registry.Namespace) > 0 {
			build.Image = fmt.Sprintf("%s/%s/%s", registry.RegAddr, registry.Namespace, imageTag)
		} else {
			build.Image = fmt.Sprintf("%s/%s", registry.RegAddr, imageTag)
		}

		build.Image = strings.TrimPrefix(build.Image, "http://")
		build.Image = strings.TrimPrefix(build.Image, "https://")

		build.Package = fmt.Sprintf("%s.tar.gz", commonservice.ReleaseCandidate(build.Repos, taskID, j.workflow.Project, build.ServiceModule, "", build.ServiceModule, "tar"))

		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
		if err != nil {
			return resp, err
		}
		if err := fillBuildDetail(buildInfo, build.ServiceName, build.ServiceModule); err != nil {
			return resp, err
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(buildInfo.PreBuild.ImageID)
		if err != nil {
			return resp, err
		}
		registries, err := commonservice.ListRegistryNamespaces("", true, logger)
		if err != nil {
			return resp, err
		}
		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			Name:    jobNameFormat(build.ServiceName + "-" + build.ServiceModule + "-" + j.job.Name),
			JobType: string(config.JobZadigBuild),
			Spec:    jobTaskSpec,
			Timeout: int64(buildInfo.Timeout),
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:         int64(buildInfo.Timeout),
			ResourceRequest: buildInfo.PreBuild.ResReq,
			ResReqSpec:      buildInfo.PreBuild.ResReqSpec,
			CustomEnvs:      renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs),
			ClusterID:       buildInfo.PreBuild.ClusterID,
			BuildOS:         basicImage.Value,
			ImageFrom:       buildInfo.PreBuild.ImageFrom,
			Registries:      registries,
		}
		clusterInfo, err := commonrepo.NewK8SClusterColl().Get(buildInfo.PreBuild.ClusterID)
		if err != nil {
			return resp, err
		}

		if clusterInfo.Cache.MediumType == "" {
			jobTaskSpec.Properties.CacheEnable = false
		} else {
			jobTaskSpec.Properties.Cache = clusterInfo.Cache
			jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
		}
		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.CustomEnvs, getBuildJobVariables(build, taskID, j.workflow.Project, j.workflow.Name, registry, logger)...)

		if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.NFSMedium {
			jobTaskSpec.Properties.CacheUserDir = renderEnv(jobTaskSpec.Properties.CacheUserDir, jobTaskSpec.Properties.Envs)
			jobTaskSpec.Properties.Cache.NFSProperties.Subpath = renderEnv(jobTaskSpec.Properties.Cache.NFSProperties.Subpath, jobTaskSpec.Properties.Envs)
		}

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
		// init git clone step
		gitStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: renderRepos(build.Repos, buildInfo.Repos)},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

		// init shell step
		dockerLoginCmd := `docker login -u "$DOCKER_REGISTRY_AK" -p "$DOCKER_REGISTRY_SK" "$DOCKER_REGISTRY_HOST" &> /dev/null`
		scripts := append([]string{dockerLoginCmd}, strings.Split(replaceWrapLine(buildInfo.Scripts), "\n")...)
		shellStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-shell",
			JobName:  jobTask.Name,
			StepType: config.StepShell,
			Spec: &step.StepShellSpec{
				Scripts: scripts,
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, shellStep)

		// init docker build step
		if buildInfo.PostBuild.DockerBuild != nil {
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
					ImageName:             build.Image,
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
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, dockerBuildStep)
		}

		// init archive step
		if buildInfo.PostBuild.FileArchive != nil && buildInfo.PostBuild.FileArchive.FileLocation != "" {
			uploads := []*step.Upload{
				{
					FilePath:        path.Join(buildInfo.PostBuild.FileArchive.FileLocation, build.Package),
					DestinationPath: path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "archive"),
				},
			}
			archiveStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-archive",
				JobName:  jobTask.Name,
				StepType: config.StepArchive,
				Spec: step.StepArchiveSpec{
					UploadDetail: uploads,
					S3:           modelS3toS3(defaultS3),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init object storage step
		if buildInfo.PostBuild.ObjectStorageUpload != nil && buildInfo.PostBuild.ObjectStorageUpload.Enabled {
			modelS3, err := commonrepo.NewS3StorageColl().Find(buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID)
			if err != nil {
				return resp, err
			}
			s3 := modelS3toS3(modelS3)
			s3.Subfolder = ""
			uploads := []*step.Upload{}
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
			for _, detail := range buildInfo.PostBuild.ObjectStorageUpload.UploadDetail {
				uploads = append(uploads, &step.Upload{
					FilePath:        detail.FilePath,
					DestinationPath: detail.DestinationPath,
				})
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init psot build shell step
		if buildInfo.PostBuild.Scripts != "" {
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
	for i, originKV := range origin {
		for _, inputKV := range input {
			if originKV.Key == inputKV.Key {
				// always use origin credential config.
				isCredential := originKV.IsCredential
				origin[i] = inputKV
				origin[i].IsCredential = isCredential
			}
		}
	}
	return origin
}

func renderRepos(input, origin []*types.Repository) []*types.Repository {
	for i, originRepo := range origin {
		for _, inputRepo := range input {
			if originRepo.RepoName == inputRepo.RepoName && originRepo.RepoOwner == inputRepo.RepoOwner {
				origin[i] = inputRepo
			}
		}
	}
	return origin
}

func replaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}

func getBuildJobVariables(build *commonmodels.ServiceAndBuild, taskID int64, project, workflowName string, registry *commonmodels.RegistryNamespace, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	ret = append(ret, getReposVariables(build.Repos)...)
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_HOST", Value: registry.RegAddr, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_AK", Value: registry.AccessKey, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "DOCKER_REGISTRY_SK", Value: registry.SecretKey, IsCredential: true})

	ret = append(ret, &commonmodels.KeyVal{Key: "TASK_ID", Value: fmt.Sprintf("%d", taskID), IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: build.ServiceModule, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PROJECT", Value: project, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "WORKFLOW", Value: workflowName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "IMAGE", Value: build.Image, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "CI", Value: "true", IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "ZADIG", Value: "true", IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d", configbase.SystemAddress(), project, workflowName, taskID)
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: build.Package, IsCredential: false})
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
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}

func fillBuildDetail(moduleBuild *commonmodels.Build, serviceName, serviceModule string) error {
	if moduleBuild.TemplateID == "" {
		return nil
	}
	buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
		ID: moduleBuild.TemplateID,
	})
	if err != nil {
		return fmt.Errorf("failed to find build template with id: %s, err: %s", moduleBuild.TemplateID, err)
	}

	moduleBuild.Timeout = buildTemplate.Timeout
	moduleBuild.PreBuild = buildTemplate.PreBuild
	moduleBuild.JenkinsBuild = buildTemplate.JenkinsBuild
	moduleBuild.Scripts = buildTemplate.Scripts
	moduleBuild.PostBuild = buildTemplate.PostBuild
	moduleBuild.SSHs = buildTemplate.SSHs
	moduleBuild.PMDeployScripts = buildTemplate.PMDeployScripts
	moduleBuild.CacheEnable = buildTemplate.CacheEnable
	moduleBuild.CacheDirType = buildTemplate.CacheDirType
	moduleBuild.CacheUserDir = buildTemplate.CacheUserDir
	moduleBuild.AdvancedSettingsModified = buildTemplate.AdvancedSettingsModified

	// repos are configured by service modules
	for _, serviceConfig := range moduleBuild.Targets {
		if serviceConfig.ServiceName == serviceName && serviceConfig.ServiceModule == serviceModule {
			moduleBuild.Repos = serviceConfig.Repos
			if moduleBuild.PreBuild == nil {
				moduleBuild.PreBuild = &commonmodels.PreBuild{}
			}
			moduleBuild.PreBuild.Envs = commonservice.MergeBuildEnvs(moduleBuild.PreBuild.Envs, serviceConfig.Envs)
			break
		}
	}
	return nil
}

func renderEnv(data string, kvs []*commonmodels.KeyVal) string {
	mapper := func(data string) string {
		for _, envar := range kvs {
			if data != envar.Key {
				continue
			}

			return envar.Value
		}

		return fmt.Sprintf("$%s", data)
	}
	return os.Expand(data, mapper)
}

func mergeRepos(templateRepos []*types.Repository, customRepos []*types.Repository) []*types.Repository {
	customRepoMap := make(map[string]*types.Repository)
	for _, repo := range customRepos {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		repoKey := strings.Join([]string{repo.Source, repo.RepoNamespace, repo.RepoName}, "/")
		customRepoMap[repoKey] = repo
	}
	for _, repo := range templateRepos {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		repoKey := strings.Join([]string{repo.Source, repo.GetRepoNamespace(), repo.RepoName}, "/")
		// user can only set default branch in custom workflow.
		if cv, ok := customRepoMap[repoKey]; ok {
			repo.Branch = cv.Branch
			repo.Tag = cv.Tag
			repo.PR = cv.PR
			repo.PRs = cv.PRs
		}
	}
	return templateRepos
}

func (j *BuildJob) LintJob() error {
	return nil
}
