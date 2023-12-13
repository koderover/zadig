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

package service

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	openapitool "github.com/koderover/zadig/v2/pkg/tool/openapi"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPICreateScanningModule(username string, args *OpenAPICreateScanningReq, log *zap.SugaredLogger) error {
	scanning, err := generateScanningModuleFromOpenAPIInput(args, log)
	if err != nil {
		log.Errorf("failed to generate scanning module from input, err: %s", err)
		return err
	}

	return CreateScanningModule(username, scanning, log)
}

func OpenAPICreateScanningTask(username string, args *OpenAPICreateScanningTaskReq, log *zap.SugaredLogger) (int64, error) {
	scan, err := mongodb.NewScanningColl().Find(args.ProjectName, args.ScanName)
	if err != nil {
		log.Errorf("failed to find scanning module, err: %s", err)
		return 0, err
	}

	scanDatas := make([]*ScanningRepoInfo, 0)
	for _, repo := range args.ScanRepos {
		for _, dbRepo := range scan.Repos {
			if repo.RepoName == dbRepo.RepoName && repo.RepoOwner == dbRepo.RepoOwner && repo.Source == dbRepo.Source {
				scanDatas = append(scanDatas, &ScanningRepoInfo{
					RepoName:      repo.RepoName,
					RepoNamespace: dbRepo.RepoNamespace,
					RepoOwner:     repo.RepoOwner,
					Source:        repo.Source,
					Branch:        repo.Branch,
					CodehostID:    dbRepo.CodehostID,
					PRs:           repo.PRs,
				})
			}
			break
		}
	}

	return CreateScanningTask(scan.ID.Hex(), scanDatas, "", username, log)
}

func generateScanningModuleFromOpenAPIInput(req *OpenAPICreateScanningReq, log *zap.SugaredLogger) (*Scanning, error) {
	ret := &Scanning{
		Name:             req.Name,
		ProjectName:      req.ProjectName,
		Description:      req.Description,
		ScannerType:      req.ScannerType,
		Installs:         req.Addons,
		Parameter:        req.SonarParameter,
		Script:           req.Script,
		CheckQualityGate: req.EnableQualityGate,
	}
	// since only one sonar system can be integrated, use that as the sonarID
	if req.ScannerType == "sonarQube" {
		sonarInfo, _, err := mongodb.NewSonarIntegrationColl().List(context.TODO(), 1, 20)
		if err != nil {
			log.Errorf("failed to list sonar integration, err is: %s", err)
			return nil, fmt.Errorf("didn't find the sonar integration to fill in")
		}
		for _, item := range sonarInfo {
			if item.SystemIdentity == req.SonarSystem {
				ret.SonarID = item.ID.Hex()
			}
		}
		if ret.SonarID == "" {
			return nil, fmt.Errorf("didn't find the sonar integration of given name")
		}

		ret.EnableScanner = true
		ret.Script = req.PrelaunchScript
	}

	// find the correct image info to fill in
	imageInfo, err := mongodb.NewBasicImageColl().FindByImageName(req.ImageName)
	if err != nil {
		log.Errorf("failed to find the image name by tag")
	}
	ret.ImageID = imageInfo.ID.Hex()

	// advanced setting conversion
	scanningAdvancedSetting, err := openapitool.ToScanningAdvancedSetting(req.AdvancedSetting)
	if err != nil {
		log.Errorf("failed to convert openAPI scanning hook info to system hook info, err: %s", err)
		return nil, err
	}
	ret.AdvancedSetting = scanningAdvancedSetting

	//repo info conversion
	repoList := make([]*types.Repository, 0)
	for _, repo := range req.RepoInfo {
		systemRepoInfo, err := openapitool.ToScanningRepository(repo)
		if err != nil {
			log.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
			return nil, fmt.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
		}
		repoList = append(repoList, systemRepoInfo)
	}

	ret.Repos = repoList
	return ret, nil
}

func OpenAPICreateTestTask(userName string, args *OpenAPICreateTestTaskReq, logger *zap.SugaredLogger) (int64, error) {
	task := &commonmodels.TestTaskArgs{
		TestName:        args.TestName,
		ProductName:     args.ProjectName,
		TestTaskCreator: userName,
	}
	result, err := CreateTestTask(task, logger)
	if err != nil {
		logger.Errorf("OpenAPI: failed to create test task, project:%s, test name:%s, err: %s", args.ProjectName, args.TestName, err)
		return 0, err
	}
	return result.TaskID, nil
}

func OpenAPIGetTestTaskResult(taskID int64, productName, testName string, logger *zap.SugaredLogger) (*OpenAPITestTaskDetail, error) {
	pipelineName := fmt.Sprintf("%s-%s", testName, "job")
	pipelineTask, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, config.TestType)
	if err != nil {
		logger.Errorf("OpenAPI: failed to get pipeline task from db, taskID:%d, pipelineName:%s, err: %v", taskID, pipelineName, err)
		return nil, fmt.Errorf("failed to get pipeline task from db, taskID:%d, pipelineName:%s, err: %v", taskID, pipelineName, err)
	}
	result := &OpenAPITestTaskDetail{
		TestName:   testName,
		TaskID:     taskID,
		Creator:    pipelineTask.TaskCreator,
		CreateTime: pipelineTask.CreateTime,
		StartTime:  pipelineTask.StartTime,
		EndTime:    pipelineTask.EndTime,
		Status:     string(pipelineTask.Status.ToLower()),
	}
	if pipelineTask.Status == config.StatusPassed {

		report, err := commonservice.GetLocalTestSuite(pipelineName, testName, setting.FunctionTest, taskID, "", config.TestType, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to get project:%s test:%s task:%d result, err: %v", productName, testName, taskID, err)
		} else {
			if report.FunctionTestSuite != nil {
				result.TestReport = &OpenAPITestReport{
					TestTotal:    report.FunctionTestSuite.Tests,
					SuccessTotal: report.FunctionTestSuite.Successes,
					FailureTotal: report.FunctionTestSuite.Failures,
					ErrorTotal:   report.FunctionTestSuite.Errors,
					SkipedTotal:  report.FunctionTestSuite.Skips,
					Time:         report.FunctionTestSuite.Time,
				}

				result.TestReport.TestCases = make([]*OpenAPITestCase, 0)
				for _, testCase := range report.FunctionTestSuite.TestCases {
					result.TestReport.TestCases = append(result.TestReport.TestCases, &OpenAPITestCase{
						Name:    testCase.Name,
						Time:    testCase.Time,
						Failure: testCase.Failure,
						Error:   testCase.Error,
					})
				}
			}
		}
	}

	return result, nil
}

func OpenAPIGetScanningTaskDetail(taskID int64, productName, scanName string, logger *zap.SugaredLogger) (*OpenAPIScanTaskDetail, error) {
	scan, err := mongodb.NewScanningColl().Find(productName, scanName)
	if err != nil {
		logger.Errorf("OpenAPI: failed to find scanning module:%s in project:%s, err: %s", scanName, productName, err)
		return nil, err
	}
	detail, err := GetScanningTaskInfo(scan.ID.Hex(), taskID, logger)
	if err != nil {
		logger.Errorf("OpenAPI: failed to get scanning task:%d detail, err: %s", taskID, err)
		return nil, err
	}

	resp := &OpenAPIScanTaskDetail{
		ScanName:   scanName,
		TaskID:     taskID,
		Creator:    detail.Creator,
		CreateTime: detail.CreateTime,
		EndTime:    detail.EndTime,
		ResultLink: detail.ResultLink,
		Status:     strings.ToLower(detail.Status),
	}
	resp.RepoInfo = make([]*OpenAPIScanRepoBrief, 0)
	for _, repo := range detail.RepoInfo {
		resp.RepoInfo = append(resp.RepoInfo, &OpenAPIScanRepoBrief{
			RepoName:     repo.RepoName,
			RepoOwner:    repo.RepoOwner,
			Source:       repo.Source,
			Address:      repo.Address,
			Branch:       repo.Branch,
			RemoteName:   repo.RemoteName,
			Hidden:       repo.Hidden,
			CheckoutPath: repo.CheckoutPath,
			SubModules:   repo.SubModules,
		})
	}

	return resp, nil
}
