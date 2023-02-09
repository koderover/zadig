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
	"strings"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"
)

type TestingJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigTestingJobSpec
}

func (j *TestingJob) Instantiate() error {
	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}
func (j *TestingJob) SetPreset() error {
	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	for _, testing := range j.spec.TestModules {
		testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
		if err != nil {
			log.Errorf("find testing: %s error: %v", testing.Name, err)
			continue
		}
		testing.Repos = mergeRepos(testingInfo.Repos, testing.Repos)
		testing.KeyVals = renderKeyVals(testing.KeyVals, testingInfo.PreTest.Envs)

	}
	j.job.Spec = j.spec
	return nil
}

func (j *TestingJob) GetRepos() ([]*types.Repository, error) {
	resp := []*types.Repository{}
	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	for _, testing := range j.spec.TestModules {
		testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
		if err != nil {
			log.Errorf("find testing: %s error: %v", testing.Name, err)
			continue
		}
		resp = append(resp, mergeRepos(testingInfo.Repos, testing.Repos)...)
	}
	return resp, nil
}

func (j *TestingJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigTestingJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigTestingJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}

		for _, testing := range j.spec.TestModules {
			for _, argsTesting := range argsSpec.TestModules {
				if testing.Name == argsTesting.Name {
					testing.Repos = mergeRepos(testing.Repos, argsTesting.Repos)
					testing.KeyVals = renderKeyVals(argsTesting.KeyVals, testing.KeyVals)
					break
				}
			}
		}
		j.job.Spec = j.spec
	}
	return nil
}

func (j *TestingJob) MergeWebhookRepo(webhookRepo *types.Repository) error {
	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	for _, testing := range j.spec.TestModules {
		testing.Repos = mergeRepos(testing.Repos, []*types.Repository{webhookRepo})
	}
	j.job.Spec = j.spec
	return nil
}

func (j *TestingJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	defaultS3, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return resp, fmt.Errorf("failed to find default s3 storage, error: %v", err)
	}

	for _, testing := range j.spec.TestModules {
		testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
		if err != nil {
			return resp, fmt.Errorf("find testing: %s error: %v", testing.Name, err)
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(testingInfo.PreTest.ImageID)
		if err != nil {
			return resp, fmt.Errorf("find basic image: %s error: %v", testingInfo.PreTest.ImageID, err)
		}
		registries, err := commonservice.ListRegistryNamespaces("", true, logger)
		if err != nil {
			return resp, fmt.Errorf("list registries error: %v", err)
		}
		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			Name:    jobNameFormat(testing.Name + "-" + j.job.Name + "-" + rand.String(5)),
			Key:     strings.Join([]string{j.job.Name, testing.Name}, "."),
			JobType: string(config.JobZadigTesting),
			Spec:    jobTaskSpec,
			Timeout: int64(testingInfo.Timeout),
			Outputs: testingInfo.Outputs,
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:             int64(testingInfo.Timeout),
			ResourceRequest:     testingInfo.PreTest.ResReq,
			ResReqSpec:          testingInfo.PreTest.ResReqSpec,
			CustomEnvs:          renderKeyVals(testing.KeyVals, testingInfo.PreTest.Envs),
			ClusterID:           testingInfo.PreTest.ClusterID,
			BuildOS:             basicImage.Value,
			ImageFrom:           testingInfo.PreTest.ImageFrom,
			Registries:          registries,
			ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, testing.ShareStorageInfo, j.workflow.Name, taskID),
		}
		clusterInfo, err := commonrepo.NewK8SClusterColl().Get(testingInfo.PreTest.ClusterID)
		if err != nil {
			return resp, fmt.Errorf("failed to find cluster: %s, error: %v", testingInfo.PreTest.ClusterID, err)
		}

		if clusterInfo.Cache.MediumType == "" {
			jobTaskSpec.Properties.CacheEnable = false
		} else {
			jobTaskSpec.Properties.Cache = clusterInfo.Cache
			jobTaskSpec.Properties.CacheEnable = testingInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = testingInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = testingInfo.CacheUserDir
		}
		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.CustomEnvs, getTestingJobVariables(testing.Repos, taskID, j.workflow.Project, j.workflow.Name, testing.ProjectName, testing.Name, logger)...)

		if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.NFSMedium {
			jobTaskSpec.Properties.CacheUserDir = renderEnv(jobTaskSpec.Properties.CacheUserDir, jobTaskSpec.Properties.Envs)
			jobTaskSpec.Properties.Cache.NFSProperties.Subpath = renderEnv(jobTaskSpec.Properties.Cache.NFSProperties.Subpath, jobTaskSpec.Properties.Envs)
		}

		// init tools install step
		tools := []*step.Tool{}
		for _, tool := range testingInfo.PreTest.Installs {
			tools = append(tools, &step.Tool{
				Name:    tool.Name,
				Version: tool.Version,
			})
		}
		toolInstallStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", testing.Name, "tool-install"),
			JobName:  jobTask.Name,
			StepType: config.StepTools,
			Spec:     step.StepToolInstallSpec{Installs: tools},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, toolInstallStep)
		// init git clone step
		gitStep := &commonmodels.StepTask{
			Name:     testing.Name + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: renderRepos(testing.Repos, testingInfo.Repos, jobTaskSpec.Properties.Envs)},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

		// init shell step
		shellStep := &commonmodels.StepTask{
			Name:     testing.Name + "-shell",
			JobName:  jobTask.Name,
			StepType: config.StepShell,
			Spec: &step.StepShellSpec{
				Scripts: append(strings.Split(replaceWrapLine(testingInfo.Scripts), "\n"), outputScript(testingInfo.Outputs)...),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, shellStep)

		// init archive html step
		if len(testingInfo.TestReportPath) > 0 {
			uploads := []*step.Upload{
				{
					FilePath:        testingInfo.TestReportPath,
					DestinationPath: path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "html"),
				},
			}
			archiveStep := &commonmodels.StepTask{
				Name:      config.TestJobHTMLReportStepName,
				JobName:   jobTask.Name,
				StepType:  config.StepArchive,
				Onfailure: true,
				Spec: step.StepArchiveSpec{
					UploadDetail: uploads,
					S3:           modelS3toS3(defaultS3),
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
		}

		// init test result storage step
		if len(testingInfo.ArtifactPaths) > 0 {
			tarArchiveStep := &commonmodels.StepTask{
				Name:      config.TestJobArchiveResultStepName,
				JobName:   jobTask.Name,
				StepType:  config.StepTarArchive,
				Onfailure: true,
				Spec: &step.StepTarArchiveSpec{
					ResultDirs: testingInfo.ArtifactPaths,
					S3DestDir:  path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "test-result"),
					FileName:   setting.ArtifactResultOut,
					DestDir:    "/tmp",
				},
			}
			if len(testingInfo.ArtifactPaths) > 1 || testingInfo.ArtifactPaths[0] != "" {
				jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
			}
		}

		// init junit report step
		if len(testingInfo.TestResultPath) > 0 {
			junitStep := &commonmodels.StepTask{
				Name:      config.TestJobJunitReportStepName,
				JobName:   jobTask.Name,
				StepType:  config.StepJunitReport,
				Onfailure: true,
				Spec: &step.StepJunitReportSpec{
					ReportDir: testingInfo.TestResultPath,
					S3DestDir: path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "junit"),
					TestName:  testing.Name,
					DestDir:   "/tmp",
					FileName:  "merged.xml",
				},
			}
			jobTaskSpec.Steps = append(jobTaskSpec.Steps, junitStep)
		}

		resp = append(resp, jobTask)
	}
	j.job.Spec = j.spec
	return resp, nil
}

func (j *TestingJob) LintJob() error {
	return nil
}

func (j *TestingJob) GetOutPuts(log *zap.SugaredLogger) []string {
	resp := []string{}
	j.spec = &commonmodels.ZadigTestingJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return resp
	}
	testNames := []string{}
	for _, testing := range j.spec.TestModules {
		testNames = append(testNames, testing.Name)
	}
	testingInfos, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{TestNames: testNames})
	if err != nil {
		log.Errorf("list testinfos error: %v", err)
		return resp
	}
	for _, testInfo := range testingInfos {
		jobKey := strings.Join([]string{j.job.Name, testInfo.Name}, ".")
		resp = append(resp, getOutputKey(jobKey, testInfo.Outputs)...)
	}
	return resp
}

func getTestingJobVariables(repos []*types.Repository, taskID int64, project, workflowName, testingProject, testingName string, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	ret = append(ret, getReposVariables(repos)...)

	ret = append(ret, &commonmodels.KeyVal{Key: "TASK_ID", Value: fmt.Sprintf("%d", taskID), IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "PROJECT", Value: project, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "TESTING_PROJECT", Value: testingProject, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "TESTING_NAME", Value: testingName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "WORKFLOW", Value: workflowName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "CI", Value: "true", IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "ZADIG", Value: "true", IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d", configbase.SystemAddress(), project, workflowName, taskID)
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	return ret
}
