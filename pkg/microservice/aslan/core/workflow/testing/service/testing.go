/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	pkgconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	tartool "github.com/koderover/zadig/v2/pkg/tool/tar"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

func CreateTesting(username string, testing *commonmodels.Testing, log *zap.SugaredLogger) error {
	if len(testing.Name) == 0 {
		return e.ErrCreateTestModule.AddDesc("empty Name")
	}
	if err := commonutil.CheckDefineResourceParam(testing.PreTest.ResReq, testing.PreTest.ResReqSpec); err != nil {
		return e.ErrCreateTestModule.AddDesc(err.Error())
	}
	err := HandleCronjob(testing, log)
	if err != nil {
		return e.ErrCreateTestModule.AddErr(err)
	}

	err = commonservice.ProcessWebhook(testing.HookCtl.Items, nil, webhook.TestingPrefix+testing.Name, log)
	if err != nil {
		return e.ErrUpdateTestModule.AddErr(err)
	}

	testing.UpdateBy = username
	err = commonrepo.NewTestingColl().Create(testing)
	if err != nil {
		log.Errorf("[Testing.Upsert] %s error: %v", testing.Name, err)
		return e.ErrCreateTestModule.AddErr(err)
	}

	return nil
}

func HandleCronjob(testing *commonmodels.Testing, log *zap.SugaredLogger) error {
	testSchedule := testing.Schedules

	if testSchedule != nil {
		testing.Schedules = nil
		testing.ScheduleEnabled = testSchedule.Enabled
		payload := commonservice.CronjobPayload{
			Name:        testing.Name,
			ProductName: testing.ProductName,
			JobType:     setting.TestingCronjob,
		}
		if testSchedule.Enabled {
			deleteList, err := workflowservice.UpdateCronjob(testing.Name, setting.TestingCronjob, testing.ProductName, testSchedule, log)
			if err != nil {
				log.Errorf("Failed to update cronjob, the error is: %v", err)
				return e.ErrUpsertCronjob.AddDesc(err.Error())
			}
			payload.Action = setting.TypeEnableCronjob
			payload.DeleteList = deleteList
			payload.JobList = testSchedule.Items
		} else {
			payload.Action = setting.TypeDisableCronjob
		}
		pl, _ := json.Marshal(payload)
		err := commonrepo.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
			Payload:   string(pl),
			QueueType: setting.TopicCronjob,
		})
		if err != nil {
			log.Errorf("Failed to publish cron to MsgQueueCommon, the error is: %v", err)
			return e.ErrUpsertCronjob.AddDesc(err.Error())
		}
	}
	return nil
}

func UpdateTesting(username string, testing *commonmodels.Testing, log *zap.SugaredLogger) error {
	if len(testing.Name) == 0 {
		return e.ErrUpdateTestModule.AddDesc("empty Name")
	}
	if err := commonutil.CheckDefineResourceParam(testing.PreTest.ResReq, testing.PreTest.ResReqSpec); err != nil {
		return e.ErrUpdateTestModule.AddDesc(err.Error())
	}
	err := HandleCronjob(testing, log)
	if err != nil {
		return e.ErrUpdateTestModule.AddErr(err)
	}

	existed, err := commonrepo.NewTestingColl().Find(testing.Name, testing.ProductName)
	if err == nil && existed.PreTest != nil && testing.PreTest != nil {
		commonservice.EnsureSecretEnvs(existed.PreTest.Envs, testing.PreTest.Envs)
	}

	err = commonservice.ProcessWebhook(testing.HookCtl.Items, existed.HookCtl.Items, webhook.TestingPrefix+testing.Name, log)
	if err != nil {
		return e.ErrUpdateTestModule.AddErr(err)
	}

	testing.UpdateBy = username
	testing.UpdateTime = time.Now().Unix()

	if err := commonrepo.NewTestingColl().Update(testing); err != nil {
		log.Errorf("[Testing.Upsert] %s error: %v", testing.Name, err)
		return e.ErrUpdateTestModule.AddErr(err)
	}

	return nil
}

type TestingOpt struct {
	Name        string                     `bson:"name"                   json:"name"`
	ProductName string                     `bson:"product_name"           json:"product_name"`
	Desc        string                     `bson:"desc"                   json:"desc"`
	UpdateTime  int64                      `bson:"update_time"            json:"update_time"`
	UpdateBy    string                     `bson:"update_by"              json:"update_by"`
	TestCaseNum int                        `bson:"-"                      json:"test_case_num,omitempty"`
	ExecuteNum  int                        `bson:"-"                      json:"execute_num,omitempty"`
	PassRate    float64                    `bson:"-"                      json:"pass_rate,omitempty"`
	AvgDuration float64                    `bson:"-"                      json:"avg_duration,omitempty"`
	Workflows   []*commonmodels.Workflow   `bson:"-"                      json:"workflows,omitempty"`
	Schedules   *commonmodels.ScheduleCtrl `bson:"-"                      json:"schedules,omitempty"`
	Repos       []*types.Repository        `bson:"repos"                  json:"repos"`
	KeyVals     []*commonmodels.KeyVal     `bson:"key_vals"               json:"key_vals"`
	ClusterID   string                     `bson:"cluster_id"             json:"cluster_id"`
	Verbs       []string                   `bson:"-"                      json:"verbs,omitempty"`
}

func ListTestingOpt(productNames []string, testType string, log *zap.SugaredLogger) ([]*TestingOpt, error) {
	allTestings := make([]*commonmodels.Testing, 0)
	// TODO: remove the test type cases, there are only function test type right now. (v2.1.0)
	if testType == "" {
		testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{TestType: testType})
		if err != nil {
			log.Errorf("TestingModule.List error: %v", err)
			return nil, e.ErrListTestModule.AddDesc(err.Error())
		}
		allTestings = append(allTestings, testings...)
	} else {
		testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductNames: productNames, TestType: testType})
		if err != nil {
			log.Errorf("[Testing.List] error: %v", err)
			return nil, e.ErrListTestModule.AddErr(err)
		}
		for _, testing := range testings {
			if testType == "function" {
				testTaskStat, _ := GetTestTask(testing.Name)
				if testTaskStat == nil {
					testTaskStat = new(commonmodels.TestTaskStat)
				}
				testing.TestCaseNum = testTaskStat.TestCaseNum
				totalNum := testTaskStat.TotalSuccess + testTaskStat.TotalFailure
				testing.ExecuteNum = totalNum
				if totalNum != 0 {
					passRate := float64(testTaskStat.TotalSuccess) / float64(totalNum)
					testing.PassRate = decimal(passRate)
					avgDuration := float64(testTaskStat.TotalDuration) / float64(totalNum)
					testing.AvgDuration = decimal(avgDuration)
				}
				// TODO: Remove the code below. After the removal of the product workflow, there will no longer be references in the testing modules.
				//testing.Workflows, _ = ListAllWorkflows(testing.Name, log)
			}
			if testing.PreTest.ConcurrencyLimit == 0 {
				testing.PreTest.ConcurrencyLimit = -1
			}
			allTestings = append(allTestings, testing)
		}
	}

	testingOpts := make([]*TestingOpt, 0)
	for _, t := range allTestings {
		testingOpts = append(testingOpts, &TestingOpt{
			Name:        t.Name,
			ProductName: t.ProductName,
			Desc:        t.Desc,
			UpdateTime:  t.UpdateTime,
			UpdateBy:    t.UpdateBy,
			TestCaseNum: t.TestCaseNum,
			ExecuteNum:  t.ExecuteNum,
			PassRate:    t.PassRate,
			AvgDuration: t.AvgDuration,
			Workflows:   t.Workflows,
			Schedules:   t.Schedules,
			Repos:       t.Repos,
			KeyVals:     t.PreTest.Envs,
			ClusterID:   t.PreTest.ClusterID,
		})
	}

	return testingOpts, nil
}

// @fixme this function has performance issue, need to support get multiple test tasks at once
func GetTestTask(testName string) (*commonmodels.TestTaskStat, error) {
	testCustomWorkflowName := commonutil.GenTestingWorkflowName(testName)
	testTasks, err := commonrepo.NewJobInfoColl().GetTestJobsByWorkflow(testCustomWorkflowName)
	if err != nil {
		log.Errorf("failed to get test task from mongodb, error: %s", err)
		return nil, err
	}

	resp := &commonmodels.TestTaskStat{
		Name: testName,
	}

	if len(testTasks) == 0 {
		return resp, nil
	}

	var success, failure int
	var duration int64
	for _, testTask := range testTasks {
		if testTask.Status == "passed" {
			success++
		} else {
			failure++
		}

		duration += testTask.Duration
	}

	resp.TotalDuration = duration
	resp.TotalFailure = failure
	resp.TotalSuccess = success

	// get latest test result to determine how many cases are there
	testResults, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(testCustomWorkflowName, testName, testTasks[0].TaskID)
	if err != nil {
		log.Errorf("failed to get test report info for test: %s, error: %s", err)
		resp.TestCaseNum = 0
		// this error can be ignored.
		return resp, nil
	}

	for _, testResult := range testResults {
		if testResult.ZadigTestName == testName {
			resp.TestCaseNum = testResult.TestCaseNum
		}
	}

	return resp, nil
}

func decimal(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return value
}

func ListAllWorkflows(testName string, log *zap.SugaredLogger) ([]*commonmodels.Workflow, error) {
	workflows, err := commonrepo.NewWorkflowColl().ListByTestName(testName)
	if err != nil {
		log.Errorf("Workflow.List error: %v", err)
		return nil, e.ErrListWorkflow.AddDesc(err.Error())
	}

	return workflows, nil
}

func GetTesting(name, productName string, log *zap.SugaredLogger) (*commonmodels.Testing, error) {
	resp, err := GetRaw(name, productName, log)
	if err != nil {
		return nil, err
	}

	// 数据兼容： 4.1.2版本之前的定时器数据已经包含在workflow的schedule字段中，而4.1.3及以后的定时器数据需要在cronjob表中获取
	if resp.Schedules == nil {
		schedules, err := ListCronjob(resp.Name, setting.TestingCronjob)
		if err != nil {
			return nil, err
		}
		scheduleList := []*commonmodels.Schedule{}

		for _, v := range schedules {
			scheduleList = append(scheduleList, &commonmodels.Schedule{
				ID:           v.ID,
				Number:       v.Number,
				Frequency:    v.Frequency,
				Time:         v.Time,
				MaxFailures:  v.MaxFailure,
				TaskArgs:     v.TaskArgs,
				WorkflowArgs: v.WorkflowArgs,
				TestArgs:     v.TestArgs,
				Type:         config.ScheduleType(v.JobType),
				Cron:         v.Cron,
				Enabled:      v.Enabled,
			})
		}
		schedule := commonmodels.ScheduleCtrl{
			Enabled: resp.ScheduleEnabled,
			Items:   scheduleList,
		}
		resp.Schedules = &schedule
	}

	if resp.PreTest != nil && resp.PreTest.StrategyID == "" {
		clusterID := resp.PreTest.ClusterID
		if clusterID == "" {
			clusterID = setting.LocalClusterID
		}
		cluster, err := commonrepo.NewK8SClusterColl().FindByID(clusterID)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, fmt.Errorf("failed to find cluster %s, error: %v", resp.PreTest.ClusterID, err)
			}
		} else if cluster.AdvancedConfig != nil {
			strategies := cluster.AdvancedConfig.ScheduleStrategy
			for _, strategy := range strategies {
				if strategy.Default {
					resp.PreTest.StrategyID = strategy.StrategyID
					break
				}
			}
		}
	}

	if resp.PreTest.ConcurrencyLimit == 0 {
		resp.PreTest.ConcurrencyLimit = -1
	}

	for _, notify := range resp.NotifyCtls {
		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
	}

	ensureTestingResp(resp)

	return resp, nil
}

func ensureTestingResp(mt *commonmodels.Testing) {
	if len(mt.Repos) == 0 {
		mt.Repos = make([]*types.Repository, 0)
	}
	for _, repo := range mt.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
	}

	if mt.PreTest != nil {
		if len(mt.PreTest.Installs) == 0 {
			mt.PreTest.Installs = make([]*commonmodels.Item, 0)
		}
		if len(mt.PreTest.Envs) == 0 {
			mt.PreTest.Envs = make([]*commonmodels.KeyVal, 0)
		}
		// 隐藏用户设置的敏感信息
		for k := range mt.PreTest.Envs {
			if mt.PreTest.Envs[k].IsCredential {
				mt.PreTest.Envs[k].Value = setting.MaskValue
			}
		}
	}
}

// GetRaw find the testing module with secret env not masked
func GetRaw(name, productName string, log *zap.SugaredLogger) (*commonmodels.Testing, error) {
	if len(name) == 0 {
		return nil, e.ErrGetTestModule.AddDesc("empty Name")
	}

	resp, err := commonrepo.NewTestingColl().Find(name, productName)
	if err != nil {
		log.Errorf("[Testing.Find] %s: error: %v", name, err)
		return nil, e.ErrGetTestModule.AddErr(err)
	}

	return resp, nil
}

func ListCronjob(name, jobType string) ([]*commonmodels.Cronjob, error) {
	return commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
		ParentName: name,
		ParentType: jobType,
	})
}

func GetWorkflowV4HTMLTestReport(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	workflowTask, err := mongodb.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return "", fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != string(config.JobZadigTesting) {
				return "", fmt.Errorf("job: %s was not a testing job", jobName)
			}
			jobTask = job
		}
	}
	if jobTask == nil {
		return "", fmt.Errorf("cannot find job task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}

	return downloadHtmlReportFromJobTask(jobTask, workflowTask.ProjectName, workflowName, taskID, log)
}

func GetTestTaskHTMLTestReport(testName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	workflowName := commonutil.GenTestingWorkflowName(testName)
	workflowTask, err := mongodb.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return "", fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == string(config.JobZadigTesting) {
				jobTask = job
				break
			}
		}
	}
	if jobTask == nil {
		return "", fmt.Errorf("cannot find job task for test task, workflow name: %s, task id: %d", workflowName, taskID)
	}

	return downloadHtmlReportFromJobTask(jobTask, workflowTask.ProjectName, workflowName, taskID, log)
}

func downloadHtmlReportFromJobTask(jobTask *commonmodels.JobTask, projectName, workflowName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		err := fmt.Errorf("unmashal job spec error: %v", err)
		log.Error(err)
		return "", err
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if step.Name != config.TestJobHTMLReportStepName {
			continue
		}
		if step.StepType != config.StepArchive && step.StepType != config.StepTarArchive {
			err := fmt.Errorf("step: %s was not a html report step", step.Name)
			log.Error(err)
			return "", err
		}
		stepTask = step
	}
	if stepTask == nil {
		err := fmt.Errorf("cannot find step task for test task, workflow name: %s, task id: %d", workflowName, taskID)
		log.Error(err)
		return "", err
	}

	htmlReportPath := pkgconfig.LocalHtmlReportPath(projectName, workflowName, jobTask.Name, taskID)

	// check if exist first
	_, err := os.Stat(htmlReportPath)
	if err != nil && !os.IsNotExist(err) {
		// err
		err = fmt.Errorf("failed to check html report file, path: %s, err: %s", htmlReportPath, err)
		log.Error(err)
		return "", e.ErrGetTestReport.AddErr(err)
	} else if err == nil {
		// exist
		return htmlReportPath, nil
	}

	// not exist
	err = os.MkdirAll(htmlReportPath, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("failed to create html report path, path: %s, err: %s", htmlReportPath, err)
		log.Error(err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	// download it from s3
	store, err := s3.FindDefaultS3()
	if err != nil {
		err = fmt.Errorf("failed to find default s3, err: %s", err)
		log.Error(err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
	if err != nil {
		log.Errorf("download html test report error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	if stepTask.StepType == config.StepArchive {
		stepSpec := &step.StepArchiveSpec{}
		if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
			return "", fmt.Errorf("unmashal step spec error: %v", err)
		}

		fpath := ""
		fname := ""
		for _, artifact := range stepSpec.UploadDetail {
			fname = path.Base(artifact.FilePath)
			fpath = filepath.Join(artifact.DestinationPath, fname)
		}

		fname = commonutil.RenderEnv(fname, jobSpec.Properties.Envs)
		fpath = commonutil.RenderEnv(fpath, jobSpec.Properties.Envs)

		objectKey := store.GetObjectPath(fpath)
		downloadDest := filepath.Join(htmlReportPath, fname)
		err = client.Download(store.Bucket, objectKey, downloadDest)
		if err != nil {
			err = fmt.Errorf("download html test report error: %s", err)
			log.Error(err)
			return "", e.ErrGetTestReport.AddErr(err)
		}

		return htmlReportPath, nil
	} else if stepTask.StepType == config.StepTarArchive {
		stepSpec := &step.StepTarArchiveSpec{}
		if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
			return "", e.ErrGetTestReport.AddErr(fmt.Errorf("unmashal step spec error: %v", err))
		}

		downloadDest := filepath.Join(htmlReportPath, setting.HtmlReportArchivedFileName)
		objectKey := filepath.Join(stepSpec.S3DestDir, stepSpec.FileName)
		objectKey = commonutil.RenderEnv(objectKey, jobSpec.Properties.Envs)

		err = client.Download(store.Bucket, objectKey, downloadDest)
		if err != nil {
			err = fmt.Errorf("download html test report error: %s", err)
			log.Error(err)
			return "", e.ErrGetTestReport.AddErr(err)
		}

		err = tartool.Untar(downloadDest, htmlReportPath, true)
		if err != nil {
			err = fmt.Errorf("Untar %s err: %v", downloadDest, err)
			log.Error(err)
			return "", e.ErrGetTestReport.AddErr(err)
		}

		if len(stepSpec.ResultDirs) == 0 {
			return "", e.ErrGetTestReport.AddErr(fmt.Errorf("not found html report step in job task"))
		}

		unTarFilePath := filepath.Join(htmlReportPath, stepSpec.ResultDirs[0])
		unTarFilePath = commonutil.RenderEnv(unTarFilePath, jobSpec.Properties.Envs)
		unTarFileInfo, err := os.Stat(unTarFilePath)
		if err != nil {
			err = fmt.Errorf("failed to stat untar files %s, err: %v", unTarFilePath, err)
			log.Error(err)
			return "", e.ErrGetTestReport.AddErr(err)
		}

		if unTarFileInfo.IsDir() {
			untarFiles, err := os.ReadDir(unTarFilePath)
			if err != nil {
				err = fmt.Errorf("failed to read files in extracted directory, path: %s, err: %s", htmlReportPath, err)
				log.Error(err)
				return "", e.ErrGetTestReport.AddErr(err)
			}

			// Batch move files
			for _, file := range untarFiles {
				oldPath := filepath.Join(unTarFilePath, file.Name())
				newPath := filepath.Join(htmlReportPath, file.Name())
				err := os.Rename(oldPath, newPath)
				if err != nil {
					err = fmt.Errorf("failed to move file from %s to %s, err: %s", oldPath, newPath, err)
					log.Error(err)
					return "", e.ErrGetTestReport.AddErr(err)
				}
			}

			err = os.Remove(unTarFilePath)
			if err != nil {
				log.Errorf("remove extracted directory %s err: %v", downloadDest, err)
			}
		}

		err = os.Remove(downloadDest)
		if err != nil {
			log.Errorf("remove download file %s err: %v", downloadDest, err)
		}

		return htmlReportPath, nil
	}

	return "", e.ErrGetTestReport.AddErr(fmt.Errorf("not found html report step in job task"))
}

func validateTestReportParam(pipelineName, pipelineType, taskIDStr, testName string, log *zap.SugaredLogger) error {
	if pipelineName == "" {
		log.Warn("pipelineName cannot be empty")
		return fmt.Errorf("pipelineName cannot be empty")
	}

	if pipelineType == "" {
		log.Warn("pipelineType cannot be empty")
		return fmt.Errorf("pipelineType cannot be empty")
	}

	if taskIDStr == "" {
		log.Warn("taskID cannot be empty")
		return fmt.Errorf("taskID cannot be empty")
	}

	if testName == "" {
		log.Warn("testName cannot be empty")
		return fmt.Errorf("testName cannot be empty")
	}

	if pipelineType != string(config.WorkflowType) && pipelineType != string(config.TestType) {
		log.Warn("pipelineType is invalid")
		return fmt.Errorf("pipelineType is invalid")
	}

	return nil
}
