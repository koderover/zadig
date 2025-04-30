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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type CreateTaskResp struct {
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
}

func CreateTestTask(args *commonmodels.TestTaskArgs, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}
	_, err := commonrepo.NewTestingColl().Find(args.TestName, args.ProductName)
	if err != nil {
		log.Errorf("find test[%s] error: %v", args.TestName, err)
		return nil, fmt.Errorf("find test[%s] error: %v", args.TestName, err)
	}

	pipelineName := fmt.Sprintf("%s-%s", args.TestName, "job")

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.TestTaskFmt, pipelineName))
	if err != nil {
		log.Errorf("CreateTestTask Counter.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	configPayload := commonservice.GetConfigPayload(0)

	defaultS3Store, err := s3.FindDefaultS3()
	if err != nil {
		err = e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
		return nil, err
	}

	defaultURL, err := defaultS3Store.GetEncrypted()
	if err != nil {
		err = e.ErrS3Storage.AddErr(err)
		return nil, err
	}

	pipelineTask := &task.Task{
		TaskID:       nextTaskID,
		PipelineName: pipelineName,
		ProductName:  args.ProductName,
	}

	stages := make([]*commonmodels.Stage, 0)
	testTask, err := workflowservice.TestArgsToTestSubtask(args, pipelineTask, log)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	workflowservice.FmtBuilds(testTask.JobCtx.Builds, log)
	testSubTask, err := testTask.ToSubTask()
	if err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	err = workflowservice.SetCandidateRegistry(configPayload, log)
	if err != nil {
		return nil, err
	}

	workflowservice.AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)

	sort.Sort(workflowservice.ByStageKind(stages))

	triggerBy := &commonmodels.TriggerBy{
		CodehostID:     args.CodehostID,
		RepoOwner:      args.RepoOwner,
		RepoName:       args.RepoName,
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
		Ref:            args.Ref,
		EventType:      args.EventType,
	}
	task := &task.Task{
		TaskID:        nextTaskID,
		Type:          config.TestType,
		ProductName:   args.ProductName,
		PipelineName:  pipelineName,
		TaskCreator:   args.TestTaskCreator,
		Status:        config.StatusCreated,
		Stages:        stages,
		TestArgs:      args,
		ConfigPayload: configPayload,
		StorageURI:    defaultURL,
		TriggerBy:     triggerBy,
	}

	if len(task.Stages) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	if err := workflowservice.CreateTask(task); err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask
	}

	_ = scmnotify.NewService().UpdateWebhookCommentForTest(task, log)
	resp := &CreateTaskResp{PipelineName: pipelineName, TaskID: nextTaskID}
	return resp, nil
}

// CreateTestTaskV2 creates a test task, but with the new custom workflow engine
func CreateTestTaskV2(args *commonmodels.TestTaskArgs, username, account, userID string, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}

	testInfo, err := commonrepo.NewTestingColl().Find(args.TestName, args.ProductName)
	if err != nil {
		log.Errorf("find test[%s] error: %v", args.TestName, err)
		return nil, fmt.Errorf("find test[%s] error: %v", args.TestName, err)
	}

	for _, repo := range testInfo.Repos {
		workflowservice.SetRepoInfo(repo, testInfo.Repos, log)
	}

	testWorkflow, err := generateCustomWorkflowFromTestingModule(testInfo, args)

	createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
		Name:    username,
		Account: account,
		UserID:  userID,
		Type:    config.WorkflowTaskTypeTesting,
	}, testWorkflow, log)

	if createResp != nil {
		return &CreateTaskResp{
			PipelineName: createResp.WorkflowName,
			TaskID:       createResp.TaskID,
		}, err
	}

	return nil, err
}

type TestTaskList struct {
	Total int                       `json:"total"`
	Tasks []*commonrepo.TaskPreview `json:"tasks"`
}

func ListTestTask(testName, projectKey string, pageNum, pageSize int, log *zap.SugaredLogger) (*TestTaskList, error) {
	workflowName := util.GenTestingWorkflowName(testName)
	workflowTasks, total, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{
		WorkflowName: workflowName,
		ProjectName:  projectKey,
		Skip:         (pageNum - 1) * pageSize,
		Limit:        pageSize,
	})

	if err != nil {
		log.Errorf("failed to find testing module task of test: %s (common workflow name: %s), error: %s", testName, workflowName, err)
		return nil, err
	}

	taskPreviewList := make([]*commonrepo.TaskPreview, 0)

	for _, testTask := range workflowTasks {
		testResultMap := make(map[string]interface{})

		testResultList, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(workflowName, testName, testTask.TaskID)
		if err != nil {
			log.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
			return nil, fmt.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
		}

		for _, testResult := range testResultList {
			mapKey := testName
			if testResult.TestName != "" {
				mapKey = testResult.TestName
			}
			testResultMap[mapKey] = &commonmodels.TestSuite{
				Tests:     testResult.TestCaseNum,
				Failures:  testResult.FailedCaseNum,
				Successes: testResult.SuccessCaseNum,
				Skips:     testResult.SkipCaseNum,
				Errors:    testResult.ErrorCaseNum,
				Time:      testResult.TestTime,
				Name:      testResult.TestName,
			}
		}

		taskPreviewList = append(taskPreviewList, &commonrepo.TaskPreview{
			TaskID:       testTask.TaskID,
			TaskCreator:  testTask.TaskCreator,
			ProductName:  projectKey,
			PipelineName: testName,
			Status:       testTask.Status,
			CreateTime:   testTask.CreateTime,
			StartTime:    testTask.StartTime,
			EndTime:      testTask.EndTime,
			TestReports:  testResultMap,
		})
	}

	return &TestTaskList{
		Total: int(total),
		Tasks: taskPreviewList,
	}, nil
}

func GetTestTaskDetail(projectKey, testName string, taskID int64, log *zap.SugaredLogger) (*task.Task, error) {
	workflowName := util.GenTestingWorkflowName(testName)
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		log.Errorf("failed to find workflow task %d for test: %s, error: %s", taskID, testName, err)
		return nil, err
	}

	testResultMap := make(map[string]interface{})

	testResultList, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(workflowName, testName, taskID)
	if err != nil {
		log.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
		return nil, fmt.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
	}

	for _, testResult := range testResultList {
		mapKey := testName
		if testResult.TestName != "" {
			mapKey = testResult.TestName
		}
		testResultMap[mapKey] = &commonmodels.TestSuite{
			Tests:     testResult.TestCaseNum,
			Failures:  testResult.FailedCaseNum,
			Successes: testResult.SuccessCaseNum,
			Skips:     testResult.SkipCaseNum,
			Errors:    testResult.ErrorCaseNum,
			Time:      testResult.TestTime,
			TestCases: testResult.TestCases,
			Name:      testResult.TestName,
		}
	}

	if len(workflowTask.WorkflowArgs.Stages) != 1 || len(workflowTask.WorkflowArgs.Stages[0].Jobs) != 1 {
		errMsg := fmt.Sprintf("invalid test task!")
		log.Errorf(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	stages := make([]*commonmodels.Stage, 0)

	subTaskInfo := make(map[string]map[string]interface{})

	args := new(commonmodels.ZadigTestingJobSpec)
	err = commonmodels.IToi(workflowTask.WorkflowArgs.Stages[0].Jobs[0].Spec, args)
	if err != nil {
		log.Errorf("failed to decode testing job args, err: %s", err)
		return nil, err
	}

	if len(args.TestModules) != 1 {
		log.Errorf("wrong test length: %d", len(args.TestModules))
		return nil, fmt.Errorf("wrong test length: %d", len(args.TestModules))
	}

	spec := new(workflowservice.ZadigTestingJobSpec)
	err = commonmodels.IToi(workflowTask.WorkflowArgs.Stages[0].Jobs[0].Spec, spec)
	if err != nil {
		log.Errorf("failed to decode testing job spec , err: %s", err)
		return nil, err
	}

	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(workflowTask.Stages[0].Jobs[0].Spec, jobSpec); err != nil {
		return nil, fmt.Errorf("unmashal job spec error: %v", err)
	}
	isHasArtifact := false
	htmlReport := false
	junitReport := false
	jobName := ""

	for _, step := range jobSpec.Steps {
		if workflowTask.Stages[0].Jobs[0].Status == config.StatusPassed || workflowTask.Stages[0].Jobs[0].Status == config.StatusFailed {
			if step.Name == config.TestJobHTMLReportStepName {
				htmlReport = true
			}
			if step.Name == config.TestJobJunitReportStepName {
				junitReport = true
			}
		}

		if step.Name == config.TestJobArchiveResultStepName {
			if step.StepType != config.StepTarArchive {
				return nil, fmt.Errorf("step: %s was not a junit report step", step.Name)
			}
			if workflowTask.Stages[0].Jobs[0].Status == config.StatusPassed || workflowTask.Stages[0].Jobs[0].Status == config.StatusFailed {
				isHasArtifact = true
			}
			jobName = step.JobName
		}
	}

	subTaskInfo[testName] = map[string]interface{}{
		"start_time": workflowTask.Stages[0].Jobs[0].StartTime,
		"end_time":   workflowTask.Stages[0].Jobs[0].EndTime,
		"status":     workflowTask.Stages[0].Jobs[0].Status,
		"job_ctx": struct {
			JobName       string              `json:"job_name"`
			IsHasArtifact bool                `json:"is_has_artifact"`
			Builds        []*types.Repository `json:"builds"`
		}{
			jobName,
			isHasArtifact,
			args.TestModules[0].Repos,
		},
		"html_report":  htmlReport,
		"junit_report": junitReport,
		"type":         "testingv2",
	}

	stages = append(stages, &commonmodels.Stage{
		TaskType:  "testingv2",
		Status:    workflowTask.Stages[0].Status,
		SubTasks:  subTaskInfo,
		StartTime: workflowTask.Stages[0].StartTime,
		EndTime:   workflowTask.Stages[0].EndTime,
		TypeName:  workflowTask.Stages[0].Name,
	})

	return &task.Task{
		TaskID:              workflowTask.TaskID,
		ProductName:         workflowTask.ProjectName,
		PipelineName:        workflowName,
		PipelineDisplayName: testName,
		Type:                "test",
		Status:              workflowTask.Status,
		TaskCreator:         workflowTask.TaskCreator,
		TaskRevoker:         workflowTask.TaskRevoker,
		CreateTime:          workflowTask.CreateTime,
		StartTime:           workflowTask.StartTime,
		EndTime:             workflowTask.EndTime,
		Stages:              stages,
		TestReports:         testResultMap,
		IsRestart:           workflowTask.IsRestart,
	}, nil
}

func GetTestTaskReportDetail(projectKey, testName string, taskID int64, log *zap.SugaredLogger) ([]*commonmodels.TestSuite, error) {
	workflowName := util.GenTestingWorkflowName(testName)
	testResults := make([]*commonmodels.TestSuite, 0)

	testResultList, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(workflowName, testName, taskID)
	if err != nil {
		log.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
		return nil, fmt.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
	}

	for _, testResult := range testResultList {
		testResults = append(testResults, &commonmodels.TestSuite{
			Tests:     testResult.TestCaseNum,
			Failures:  testResult.FailedCaseNum,
			Successes: testResult.SuccessCaseNum,
			Skips:     testResult.SkipCaseNum,
			Errors:    testResult.ErrorCaseNum,
			Time:      testResult.TestTime,
			TestCases: testResult.TestCases,
			Name:      testResult.TestName,
		})
	}

	return testResults, nil
}

func generateCustomWorkflowFromTestingModule(testInfo *commonmodels.Testing, args *commonmodels.TestTaskArgs) (*commonmodels.WorkflowV4, error) {
	concurrencyLimit := 1
	if testInfo.PreTest != nil {
		concurrencyLimit = testInfo.PreTest.ConcurrencyLimit
	}
	// compatibility code
	if concurrencyLimit == 0 {
		concurrencyLimit = -1
	}

	for _, notify := range testInfo.NotifyCtls {
		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return nil, err
		}
	}

	resp := &commonmodels.WorkflowV4{
		Name:             util.GenTestingWorkflowName(testInfo.Name),
		DisplayName:      testInfo.Name,
		Stages:           nil,
		Project:          testInfo.ProductName,
		CreatedBy:        "system",
		ConcurrencyLimit: concurrencyLimit,
		NotifyCtls:       testInfo.NotifyCtls,
		NotificationID:   args.NotificationID,
	}

	pr, _ := strconv.Atoi(args.MergeRequestID)
	for i, build := range testInfo.Repos {
		if build.Source == args.Source && build.RepoOwner == args.RepoOwner && build.RepoName == args.RepoName {
			testInfo.Repos[i].PR = pr
			if pr != 0 {
				testInfo.Repos[i].PRs = []int{pr}
			}
			if args.Branch != "" {
				testInfo.Repos[i].Branch = args.Branch
			}
		}
	}

	stage := make([]*commonmodels.WorkflowStage, 0)

	job := make([]*commonmodels.Job, 0)
	job = append(job, &commonmodels.Job{
		Name:    strings.ToLower(testInfo.Name),
		JobType: config.JobZadigTesting,
		Skipped: false,
		Spec: &commonmodels.ZadigTestingJobSpec{
			TestType:        "",
			Source:          config.SourceRuntime,
			JobName:         "",
			OriginJobName:   "",
			DefaultServices: nil,
			TestModules: []*commonmodels.TestModule{
				{
					Name:        testInfo.Name,
					ProjectName: testInfo.ProductName,
					KeyVals:     testInfo.PreTest.Envs.ToRuntimeList(),
					Repos:       testInfo.Repos,
				},
			},
			ServiceAndTests: nil,
		},
		RunPolicy:      "",
		ServiceModules: nil,
	})

	stage = append(stage, &commonmodels.WorkflowStage{
		Name:     "test",
		Parallel: false,
		Jobs:     job,
	})

	resp.Stages = stage
	resp.HookPayload = args.HookPayload

	return resp, nil
}
