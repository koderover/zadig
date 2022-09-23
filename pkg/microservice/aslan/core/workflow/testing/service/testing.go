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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util"
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
			JobType:     config.TestingCronjob,
		}
		if testSchedule.Enabled {
			deleteList, err := workflowservice.UpdateCronjob(testing.Name, config.TestingCronjob, testing.ProductName, testSchedule, log)
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
		err := nsq.Publish(setting.TopicCronjob, pl)
		if err != nil {
			log.Errorf("Failed to publish to nsq topic: %s, the error is: %v", setting.TopicCronjob, err)
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
}

func ListTestingOpt(productNames []string, testType string, log *zap.SugaredLogger) ([]*TestingOpt, error) {
	allTestings := make([]*commonmodels.Testing, 0)
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

				testing.Workflows, _ = ListAllWorkflows(testing.Name, log)
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
		})
	}

	return testingOpts, nil
}

func GetTestTask(testName string) (*commonmodels.TestTaskStat, error) {
	return commonrepo.NewTestTaskStatColl().FindTestTaskStat(&commonrepo.TestTaskStatOption{Name: testName})
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
		schedules, err := ListCronjob(resp.Name, config.TestingCronjob)
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

	workflowservice.EnsureTestingResp(resp)

	return resp, nil
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

func GetHTMLTestReport(pipelineName, pipelineType, taskIDStr, testName string, log *zap.SugaredLogger) (string, error) {
	if err := validateTestReportParam(pipelineName, pipelineType, taskIDStr, testName, log); err != nil {
		return "", e.ErrGetTestReport.AddErr(err)
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		log.Errorf("invalid taskID: %s, err: %s", taskIDStr, err)
		return "", e.ErrGetTestReport.AddDesc("invalid taskID")
	}

	task, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, config.PipelineType(pipelineType))
	if err != nil {
		log.Errorf("find task failed, pipelineName: %s, type: %s, taskID: %s, err: %s", pipelineName, config.PipelineType(pipelineType), taskIDStr, err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	store, err := s3.NewS3StorageFromEncryptedURI(task.StorageURI)
	if err != nil {
		log.Errorf("parse storageURI failed, err: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, pipelineName, taskID, "test")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "test")
	}

	fileName := fmt.Sprintf("%s-%s-%s-%s-%s-html", pipelineType, pipelineName, taskIDStr, config.TaskTestingV2, testName)
	fileName = strings.Replace(strings.ToLower(fileName), "_", "-", -1)

	tmpFilename, err := util.GenerateTmpFile()
	if err != nil {
		log.Errorf("generate temp file error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}
	defer func() {
		_ = os.Remove(tmpFilename)
	}()
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("download html test report error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}
	objectKey := store.GetObjectPath(fileName)
	err = client.Download(store.Bucket, objectKey, tmpFilename)
	if err != nil {
		log.Errorf("download html test report error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	content, err := os.ReadFile(tmpFilename)
	if err != nil {
		log.Errorf("parse test report file error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	return string(content), nil
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
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		return "", fmt.Errorf("unmashal job spec error: %v", err)
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if step.Name != config.TestJobHTMLReportStepName {
			continue
		}
		if step.StepType != config.StepArchive {
			return "", fmt.Errorf("step: %s was not a junit report step", step.Name)
		}
		stepTask = step
	}
	if stepTask == nil {
		return "", fmt.Errorf("cannot find step task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	stepSpec := &step.StepArchiveSpec{}
	if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
		return "", fmt.Errorf("unmashal step spec error: %v", err)
	}
	filePath := ""
	for _, artifact := range stepSpec.UploadDetail {
		fileName := path.Base(artifact.FilePath)
		filePath = filepath.Join(artifact.DestinationPath, fileName)
	}

	store, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("parse storageURI failed, err: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	tmpFilename, err := util.GenerateTmpFile()
	if err != nil {
		log.Errorf("generate temp file error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}
	defer func() {
		_ = os.Remove(tmpFilename)
	}()
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("download html test report error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}
	objectKey := store.GetObjectPath(filePath)
	err = client.Download(store.Bucket, objectKey, tmpFilename)
	if err != nil {
		log.Errorf("download html test report error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	content, err := os.ReadFile(tmpFilename)
	if err != nil {
		log.Errorf("parse test report file error: %s", err)
		return "", e.ErrGetTestReport.AddErr(err)
	}

	return string(content), nil
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
