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
	"path"
	"strconv"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types/step"
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
			return err
		}
		for _, target := range buildInfo.Targets {
			if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
				build.Repos = buildInfo.Repos
				build.KeyVals = renderKeyVals(build.KeyVals, buildInfo.PreBuild.Envs)
				break
			}
		}
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BuildJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	registry, err := commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{ID: j.spec.DockerRegistryID})
	if err != nil {
		return resp, err
	}

	for _, build := range j.spec.ServiceAndBuilds {
		imageTag := ""
		for _, repo := range build.Repos {
			imageTag = repo.GetReleaseCandidateTag(taskID)
			if len(registry.Namespace) > 0 {
				build.Image = fmt.Sprintf("%s/%s/%s:%s", registry.RegAddr, registry.Namespace, build.ServiceModule, imageTag)
			}
			build.Image = fmt.Sprintf("%s/%s:%s", registry.RegAddr, build.ServiceModule, imageTag)
			build.Image = strings.TrimPrefix(build.Image, "http://")
			build.Image = strings.TrimPrefix(build.Image, "https://")
			break
		}
		build.Package = fmt.Sprintf("%s-%s-%d.tar.gz", build.ServiceModule, time.Now().Format("20060102150405"), taskID)

		jobTask := &commonmodels.JobTask{
			Name:    j.job.Name + "-" + build.ServiceName,
			JobType: string(config.JobZadigBuild),
		}
		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
		if err != nil {
			return resp, err
		}
		jobTask.Properties = commonmodels.JobProperties{
			Timeout:         int64(buildInfo.Timeout),
			ResourceRequest: buildInfo.PreBuild.ResReq,
			Args:            buildInfo.PreBuild.Envs,
			ClusterID:       buildInfo.PreBuild.ClusterID,
		}
		jobTask.Properties.Args = append(jobTask.Properties.Args, getJobVariables(build, taskID, j.workflow.Project, j.workflow.Name)...)

		// init tools install step
		for _, tool := range buildInfo.PreBuild.Installs {
			toolInstallStep := &commonmodels.StepTask{
				Name:     fmt.Sprintf("%s-%s-%s", build.ServiceName, tool.Name, tool.Version),
				JobName:  jobTask.Name,
				StepType: config.StepTools,
				Spec:     step.StepToolInstallSpec{Name: tool.Name, Version: tool.Version},
			}
			jobTask.Steps = append(jobTask.Steps, toolInstallStep)
		}
		// init git clone step
		gitStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: build.Repos},
		}
		jobTask.Steps = append(jobTask.Steps, gitStep)

		// init shell step
		shellStep := &commonmodels.StepTask{
			Name:     build.ServiceName + "-shell",
			JobName:  jobTask.Name,
			StepType: config.StepShell,
			Spec:     step.StepShellSpec{Scripts: strings.Split(replaceWrapLine(buildInfo.Scripts), "\n")},
		}
		jobTask.Steps = append(jobTask.Steps, shellStep)

		// init docker build step
		if buildInfo.PostBuild.DockerBuild != nil {
			dockefileContent := ""
			if buildInfo.PostBuild.DockerBuild.TemplateID != "" {
				dockerfileTemplate, err := commonrepo.NewDockerfileTemplateColl().GetById(buildInfo.PostBuild.DockerBuild.TemplateID)
				if err != nil {
					return resp, nil
				}
				dockefileContent = dockerfileTemplate.Content
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
				},
			}
			jobTask.Steps = append(jobTask.Steps, dockerBuildStep)
		}

		// init archive step
		if buildInfo.PostBuild.FileArchive != nil && buildInfo.PostBuild.FileArchive.FileLocation != "" {
			archiveStep := &commonmodels.StepTask{
				Name:     build.ServiceName + "-archive",
				JobName:  jobTask.Name,
				StepType: config.StepArchive,
				Spec: step.StepArchiveSpec{
					FilePath:        path.Join(buildInfo.PostBuild.FileArchive.FileLocation, build.Package),
					DestinationPath: path.Join(j.workflow.Name, fmt.Sprint(taskID), j.job.Name, "archive"),
				},
			}
			jobTask.Steps = append(jobTask.Steps, archiveStep)
		}

		// init object storage step
		if buildInfo.PostBuild.ObjectStorageUpload != nil && buildInfo.PostBuild.ObjectStorageUpload.Enabled {
			for _, detail := range buildInfo.PostBuild.ObjectStorageUpload.UploadDetail {
				archiveStep := &commonmodels.StepTask{
					Name:     build.ServiceName + "-archive",
					JobName:  jobTask.Name,
					StepType: config.StepArchive,
					Spec: step.StepArchiveSpec{
						FilePath:        detail.FilePath,
						DestinationPath: detail.DestinationPath,
						S3StorageID:     buildInfo.PostBuild.ObjectStorageUpload.ObjectStorageID,
					},
				}
				jobTask.Steps = append(jobTask.Steps, archiveStep)
			}
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
				origin[i] = inputKV
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

func getJobVariables(build *commonmodels.ServiceAndBuild, taskID int64, project, workflowName string) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for index, repo := range build.Repos {

		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoNameIndex), Value: repo.RepoName, IsCredential: false})

		repoName := strings.Replace(repo.RepoName, "-", "_", -1)

		repoIndex := fmt.Sprintf("REPO_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoIndex), Value: repoName, IsCredential: false})

		if len(repo.Branch) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_BRANCH", repoName), Value: repo.Branch, IsCredential: false})
		}

		if len(repo.Tag) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_TAG", repoName), Value: repo.Tag, IsCredential: false})
		}

		if repo.PR > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strconv.Itoa(repo.PR), IsCredential: false})
		}

		if len(repo.CommitID) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_COMMIT_ID", repoName), Value: repo.CommitID, IsCredential: false})
		}
	}
	ret = append(ret, &commonmodels.KeyVal{Key: "TASK_ID", Value: fmt.Sprintf("%d", taskID), IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: build.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "IMAGE", Value: build.Image, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "CI", Value: "true", IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "ZADIG", Value: "true", IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/%s/%d", configbase.SystemAddress(), project, workflowName, taskID)
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: build.Package, IsCredential: false})
	return ret
}
