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
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type TestingJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigTestingJobSpec
}

func CreateTestingJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigTestingJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return TestingJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j TestingJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j TestingJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j TestingJobController) Validate(isExecution bool) error {
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

func (j TestingJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigTestingJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.TestType = currJobSpec.TestType
	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.JobName = currJobSpec.JobName
	j.jobSpec.OriginJobName = currJobSpec.OriginJobName
	j.jobSpec.RefRepos = currJobSpec.RefRepos

	return nil
}

func (j TestingJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	newTestingRange := make([]*commonmodels.ServiceAndTest, 0)
	for _, svcTest := range j.jobSpec.ServiceAndTests {
		if ticket.IsAllowedService(j.workflow.Project, svcTest.ServiceName, svcTest.ServiceModule) {
			newTestingRange = append(newTestingRange, svcTest)
		}
	}

	j.jobSpec.ServiceAndTests = newTestingRange
	return nil
}

func (j TestingJobController) ClearOptions() {
	return
}

func (j TestingJobController) ClearSelection() {
	j.jobSpec.TargetServices = make([]*commonmodels.ServiceTestTarget, 0)
	j.jobSpec.TestModules = make([]*commonmodels.TestModule, 0)
	return
}

func (j TestingJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := make([]*commonmodels.JobTask, 0)

	defaultS3, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return resp, fmt.Errorf("failed to find default s3 storage, error: %v", err)
	}

	if j.jobSpec.TestType == config.ProductTestType {
		for jobSubTaskID, testing := range j.jobSpec.TestModules {
			jobTask, err := j.toJobTask(jobSubTaskID, testing, defaultS3, taskID, "", "", "", logger)
			if err != nil {
				return resp, err
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

		referredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		targets, err := j.getReferredJobTargets(referredJob)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", referredJob, err)
		}
		// clear service and image list to prevent old data from remaining
		j.jobSpec.TargetServices = targets
	}

	if j.jobSpec.TestType == config.ServiceTestType {
		jobSubTaskID := 0
		for _, target := range j.jobSpec.TargetServices {
			for _, testing := range j.jobSpec.ServiceAndTests {
				if testing.ServiceName != target.ServiceName || testing.ServiceModule != target.ServiceModule {
					continue
				}
				jobTask, err := j.toJobTask(jobSubTaskID, &testing.TestModule, defaultS3, taskID, string(j.jobSpec.TestType), testing.ServiceName, testing.ServiceModule, logger)
				if err != nil {
					return resp, err
				}
				jobSubTaskID++
				resp = append(resp, jobTask)
			}
		}
	}

	return resp, nil
}

func (j TestingJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j TestingJobController) SetRepoCommitInfo() error {
	return nil
}

func (j TestingJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j TestingJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j TestingJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j TestingJobController) getReferredJobTargets(jobName string) ([]*commonmodels.ServiceTestTarget, error) {
	servicetargets := make([]*commonmodels.ServiceTestTarget, 0)
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
				servicetargets = testingSpec.TargetServices
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
	return nil, fmt.Errorf("TestingJob: refered job %s not found", jobName)
}

func (j TestingJobController) toJobTask(jobSubTaskID int, testing *commonmodels.TestModule, defaultS3 *commonmodels.S3Storage, taskID int64, testType, serviceName, serviceModule string, logger *zap.SugaredLogger) (*commonmodels.JobTask, error) {
	testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
	if err != nil {
		return nil, fmt.Errorf("find testing: %s error: %v", testing.Name, err)
	}
	basicImage, err := commonrepo.NewBasicImageColl().Find(testingInfo.PreTest.ImageID)
	if err != nil {
		return nil, fmt.Errorf("find basic image: %s error: %v", testingInfo.PreTest.ImageID, err)
	}
	registries, err := commonservice.ListRegistryNamespaces("", true, logger)
	if err != nil {
		return nil, fmt.Errorf("list registries error: %v", err)
	}
	randStr := rand.String(5)
	jobKey := genJobKey(j.name, testing.Name)
	jobName := GenJobName(j.workflow, j.name, jobSubTaskID)
	jobDisplayName := genJobDisplayName(j.name, testing.Name)
	jobInfo := map[string]string{
		JobNameKey:     j.name,
		"test_type":    testType,
		"testing_name": testing.Name,
		"rand_str":     randStr,
	}

	customEnvs := applyKeyVals(testing.KeyVals, testingInfo.PreTest.Envs.ToRuntimeList(), false).ToKVList()
	if testType == string(config.ServiceTestType) {
		jobDisplayName = genJobDisplayName(j.name, serviceName, serviceModule)
		jobKey = genJobKey(j.name, testing.Name, serviceName, serviceModule)
		jobInfo = map[string]string{
			JobNameKey:       j.name,
			"test_type":      testType,
			"service_name":   serviceName,
			"service_module": serviceModule,
		}

		customEnvs, err = renderServiceVariables(j.workflow, customEnvs, serviceName, serviceModule)
		if err != nil {
			return nil, fmt.Errorf("failed to render service variables, error: %v", err)
		}
	}

	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
	jobTask := &commonmodels.JobTask{
		Key:            jobKey,
		Name:           jobName,
		DisplayName:    jobDisplayName,
		OriginName:     j.name,
		JobInfo:        jobInfo,
		JobType:        string(config.JobZadigTesting),
		Spec:           jobTaskSpec,
		Timeout:        int64(testingInfo.Timeout),
		Outputs:        testingInfo.Outputs,
		Infrastructure: testingInfo.Infrastructure,
		VMLabels:       testingInfo.VMLabels,
		ErrorPolicy:    j.errorPolicy,
	}
	jobTaskSpec.Properties = commonmodels.JobProperties{
		Timeout:             int64(testingInfo.Timeout),
		ResourceRequest:     testingInfo.PreTest.ResReq,
		ResReqSpec:          testingInfo.PreTest.ResReqSpec,
		CustomEnvs:          customEnvs,
		ClusterID:           testingInfo.PreTest.ClusterID,
		StrategyID:          testingInfo.PreTest.StrategyID,
		BuildOS:             basicImage.Value,
		ImageFrom:           testingInfo.PreTest.ImageFrom,
		Registries:          registries,
		ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, testing.ShareStorageInfo, j.workflow.Name, taskID),
		CustomLabels:        testingInfo.PreTest.CustomLabels,
		CustomAnnotations:   testingInfo.PreTest.CustomAnnotations,
	}

	cacheS3 := &commonmodels.S3Storage{}
	clusterInfo, err := commonrepo.NewK8SClusterColl().Get(testingInfo.PreTest.ClusterID)
	if err != nil {
		return jobTask, fmt.Errorf("failed to find cluster: %s, error: %v", testingInfo.PreTest.ClusterID, err)
	}

	if jobTask.Infrastructure == setting.JobVMInfrastructure {
		jobTaskSpec.Properties.CacheEnable = testingInfo.CacheEnable
		jobTaskSpec.Properties.CacheDirType = testingInfo.CacheDirType
		jobTaskSpec.Properties.CacheUserDir = testingInfo.CacheUserDir
	} else {
		if clusterInfo.Cache.MediumType == "" {
			jobTaskSpec.Properties.CacheEnable = false
		} else {
			jobTaskSpec.Properties.Cache = clusterInfo.Cache
			jobTaskSpec.Properties.CacheEnable = testingInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = testingInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = testingInfo.CacheUserDir

			if jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
				cacheS3, err = commonrepo.NewS3StorageColl().Find(jobTaskSpec.Properties.Cache.ObjectProperties.ID)
				if err != nil {
					return jobTask, fmt.Errorf("find cache s3 storage: %s error: %v", jobTaskSpec.Properties.Cache.ObjectProperties.ID, err)
				}
			}
		}
		if jobTaskSpec.Properties.CacheEnable {
			jobTaskSpec.Properties.CacheUserDir = commonutil.RenderEnv(jobTaskSpec.Properties.CacheUserDir, jobTaskSpec.Properties.Envs)
			if jobTaskSpec.Properties.Cache.MediumType == types.NFSMedium {
				jobTaskSpec.Properties.Cache.NFSProperties.Subpath = commonutil.RenderEnv(jobTaskSpec.Properties.Cache.NFSProperties.Subpath, jobTaskSpec.Properties.Envs)
			} else if jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
				cacheS3, err = commonrepo.NewS3StorageColl().Find(jobTaskSpec.Properties.Cache.ObjectProperties.ID)
				if err != nil {
					return jobTask, fmt.Errorf("find cache s3 storage: %s error: %v", jobTaskSpec.Properties.Cache.ObjectProperties.ID, err)
				}
			}
		}
	}

	paramEnvs := generateKeyValsFromWorkflowParam(j.workflow.Params)
	envs := mergeKeyVals(jobTaskSpec.Properties.CustomEnvs, paramEnvs)

	jobTaskSpec.Properties.Envs = append(envs, getTestingJobVariables(testing.Repos, taskID, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, testing.ProjectName, testing.Name, testType, serviceName, serviceModule, jobTask.Infrastructure, logger)...)

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
	// init download object cache step
	if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
		cacheDir := "/workspace"
		if jobTaskSpec.Properties.CacheDirType == types.UserDefinedCacheDir {
			cacheDir = jobTaskSpec.Properties.CacheUserDir
		}
		downloadArchiveStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", testing.Name, "download-archive"),
			JobName:  jobTask.Name,
			StepType: config.StepDownloadArchive,
			Spec: step.StepDownloadArchiveSpec{
				UnTar:      true,
				IgnoreErr:  true,
				FileName:   setting.TestingOSSCacheFileName,
				ObjectPath: getTestingJobCacheObjectPath(j.workflow.Name, testing.Name),
				DestDir:    cacheDir,
				S3:         modelS3toS3(cacheS3),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArchiveStep)
	}
	repos := applyRepos(testingInfo.Repos, testing.Repos)
	renderRepos(repos, jobTaskSpec.Properties.Envs)
	gitRepos, p4Repos := splitReposByType(repos)

	codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("find %s project codehost error: %v", j.workflow.Project, err)
	}

	// init git clone step
	gitStep := &commonmodels.StepTask{
		Name:     testing.Name + "-git",
		JobName:  jobTask.Name,
		StepType: config.StepGit,
		Spec:     step.StepGitSpec{Repos: gitRepos, CodeHosts: codehosts},
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

	p4Step := &commonmodels.StepTask{
		Name:     testing.Name + "-perforce",
		JobName:  jobTask.Name,
		StepType: config.StepPerforce,
		Spec:     step.StepP4Spec{Repos: p4Repos},
	}

	jobTaskSpec.Steps = append(jobTaskSpec.Steps, p4Step)

	// init debug before step
	debugBeforeStep := &commonmodels.StepTask{
		Name:     testing.Name + "-debug_before",
		JobName:  jobTask.Name,
		StepType: config.StepDebugBefore,
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)

	scriptStep := &commonmodels.StepTask{
		JobName: jobTask.Name,
	}
	if testingInfo.ScriptType == types.ScriptTypeShell || testingInfo.ScriptType == "" {
		scriptStep.Name = testing.Name + "-shell"
		scriptStep.StepType = config.StepShell
		scriptStep.Spec = &step.StepShellSpec{
			Scripts: append(strings.Split(replaceWrapLine(testingInfo.Scripts), "\n"), outputScript(testingInfo.Outputs, jobTask.Infrastructure)...),
		}
	} else if testingInfo.ScriptType == types.ScriptTypeBatchFile {
		scriptStep.Name = testing.Name + "-batchfile"
		scriptStep.StepType = config.StepBatchFile
		scriptStep.Spec = &step.StepBatchFileSpec{
			Scripts: append(strings.Split(replaceWrapLine(testingInfo.Scripts), "\n"), outputScript(testingInfo.Outputs, jobTask.Infrastructure)...),
		}
	} else if testingInfo.ScriptType == types.ScriptTypePowerShell {
		scriptStep.Name = testing.Name + "-powershell"
		scriptStep.StepType = config.StepPowerShell
		scriptStep.Spec = &step.StepPowerShellSpec{
			Scripts: append(strings.Split(replaceWrapLine(testingInfo.Scripts), "\n"), outputScript(testingInfo.Outputs, jobTask.Infrastructure)...),
		}
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)
	// init debug after step
	debugAfterStep := &commonmodels.StepTask{
		Name:     testing.Name + "-debug_after",
		JobName:  jobTask.Name,
		StepType: config.StepDebugAfter,
	}
	jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)

	tarDestDir := "/tmp"
	if testingInfo.ScriptType == types.ScriptTypeBatchFile {
		tarDestDir = "%TMP%"
	} else if testingInfo.ScriptType == types.ScriptTypePowerShell {
		tarDestDir = "%TMP%"
	}

	// init archive html step
	if len(testingInfo.TestReportPath) > 0 {
		testReportDir := filepath.Dir(testingInfo.TestReportPath)
		testReportName := filepath.Base(testingInfo.TestReportPath)
		tarArchiveStep := &commonmodels.StepTask{
			Name:      config.TestJobHTMLReportStepName,
			JobName:   jobTask.Name,
			StepType:  config.StepTarArchive,
			Onfailure: true,
			Spec: &step.StepTarArchiveSpec{
				FileName:     setting.HtmlReportArchivedFileName,
				AbsResultDir: true,
				ResultDirs:   []string{testReportName},
				ChangeTarDir: true,
				TarDir:       "$WORKSPACE/" + testReportDir,
				DestDir:      tarDestDir,
				S3DestDir:    path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "html-report"),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
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
				DestDir:    tarDestDir,
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
				SourceWorkflow: j.workflow.Name,
				SourceJobKey:   j.name,
				JobTaskName:    jobName,
				TaskID:         taskID,
				ReportDir:      testingInfo.TestResultPath,
				S3DestDir:      path.Join(j.workflow.Name, fmt.Sprint(taskID), jobTask.Name, "junit"),
				TestName:       testing.Name,
				TestProject:    testing.ProjectName,
				DestDir:        tarDestDir,
				FileName:       "merged.xml",
				ServiceName:    serviceName,
				ServiceModule:  serviceModule,
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, junitStep)
	}

	// init object cache step
	if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.ObjectMedium {
		cacheDir := "/workspace"
		if jobTaskSpec.Properties.CacheDirType == types.UserDefinedCacheDir {
			cacheDir = jobTaskSpec.Properties.CacheUserDir
		}
		tarArchiveStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", testing.Name, "tar-archive"),
			JobName:  jobTask.Name,
			StepType: config.StepTarArchive,
			Spec: step.StepTarArchiveSpec{
				FileName:     setting.TestingOSSCacheFileName,
				ResultDirs:   []string{"."},
				AbsResultDir: true,
				TarDir:       cacheDir,
				ChangeTarDir: true,
				S3DestDir:    getTestingJobCacheObjectPath(j.workflow.Name, testing.Name),
				IgnoreErr:    true,
				S3Storage:    modelS3toS3(cacheS3),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, tarArchiveStep)
	}

	// init object storage step
	if testingInfo.PostTest != nil && testingInfo.PostTest.ObjectStorageUpload != nil && testingInfo.PostTest.ObjectStorageUpload.Enabled {
		modelS3, err := commonrepo.NewS3StorageColl().Find(testingInfo.PostTest.ObjectStorageUpload.ObjectStorageID)
		if err != nil {
			return jobTask, fmt.Errorf("find object storage: %s failed, err: %v", testingInfo.PostTest.ObjectStorageUpload.ObjectStorageID, err)
		}
		s3 := modelS3toS3(modelS3)
		s3.Subfolder = ""
		uploads := []*step.Upload{}
		for _, detail := range testingInfo.PostTest.ObjectStorageUpload.UploadDetail {
			uploads = append(uploads, &step.Upload{
				FilePath:        detail.FilePath,
				DestinationPath: detail.DestinationPath,
			})
		}
		archiveStep := &commonmodels.StepTask{
			Name:     config.TestJobObjectStorageStepName,
			JobName:  jobTask.Name,
			StepType: config.StepArchive,
			Spec: step.StepArchiveSpec{
				UploadDetail:    uploads,
				ObjectStorageID: testingInfo.PostTest.ObjectStorageUpload.ObjectStorageID,
				S3:              s3,
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, archiveStep)
	}
	return jobTask, nil
}

func getTestingJobCacheObjectPath(workflowName, testingName string) string {
	return fmt.Sprintf("%s/cache/%s", workflowName, testingName)
}

// internal use only
func getTestingJobVariables(repos []*types.Repository, taskID int64, project, workflowName, workflowDisplayName, testingProject, testingName, testType, serviceName, serviceModule, infrastructure string, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, prepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)
	// repo envs
	ret = append(ret, getReposVariables(repos)...)

	ret = append(ret, &commonmodels.KeyVal{Key: "TESTING_PROJECT", Value: testingProject, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "TESTING_NAME", Value: testingName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "TESTING_TYPE", Value: testType, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: serviceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: serviceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: serviceModule, IsCredential: false})
	buildURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), project, workflowName, taskID, url.QueryEscape(workflowDisplayName))
	ret = append(ret, &commonmodels.KeyVal{Key: "BUILD_URL", Value: buildURL, IsCredential: false})
	return ret
}
