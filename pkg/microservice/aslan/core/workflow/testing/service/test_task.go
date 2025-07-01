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
	"strconv"
	"strings"

	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
)

type CreateTaskResp struct {
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
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
