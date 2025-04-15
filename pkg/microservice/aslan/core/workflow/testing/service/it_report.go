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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
)

func GetTestLocalTestSuite(testName string, log *zap.SugaredLogger) (*commonmodels.TestSuite, error) {
	resp := new(commonmodels.TestSuite)

	testCustomWorkflowName := commonutil.GenTestingWorkflowName(testName)
	testTasks, err := commonrepo.NewJobInfoColl().GetTestJobsByWorkflow(testCustomWorkflowName)
	if err != nil {
		log.Errorf("failed to get test task from mongodb, error: %s", err)
		return nil, err
	}

	if len(testTasks) == 0 {
		return resp, nil
	}

	// get latest test result to determine how many cases are there
	testResults, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(testCustomWorkflowName, testName, testTasks[0].TaskID)
	if err != nil {
		log.Errorf("failed to get test report info for test: %s, error: %s", err)
		return nil, err
	}

	for _, testResult := range testResults {
		if testResult.ZadigTestName == testName {
			resp.Name = testName
			resp.Tests = testResult.TestCaseNum
			resp.Skips = testResult.SkipCaseNum
			resp.Failures = testResult.FailedCaseNum
			resp.Errors = testResult.ErrorCaseNum
			resp.Time = testResult.TestTime
			resp.TestCases = testResult.TestCases
			break
		}
	}

	return resp, nil
}
