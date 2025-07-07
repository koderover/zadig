/*
Copyright 2023 The KodeRover Authors.

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

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
)

func ListTesting(uid string, pageNum, pageSize int, query string, log *zap.SugaredLogger) ([]*TestingOpt, error) {
	testingResp := make([]*TestingOpt, 0)
	allTestings := make([]*commonmodels.Testing, 0)
	testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{TestType: "function", PageNum: pageNum, PageSize: pageSize, NameQuery: query})
	if err != nil {
		log.Errorf("[Testing.List] error: %v", err)
		return nil, fmt.Errorf("list testing error: %v", err)
	}

	for _, testing := range testings {

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

		allTestings = append(allTestings, testing)
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
		})
	}
	allPermissionInfo, err := user.New().GetUserAuthInfo(uid)
	if err != nil {
		log.Errorf("failed to get user permission info, error:%s", err)
		return testingResp, err
	}
	respMap := make(map[string]*TestingOpt)
	testingProjectMap := make(map[string][]*TestingOpt)
	for _, testing := range testingOpts {
		testingProjectMap[testing.ProductName] = append(testingProjectMap[testing.ProductName], testing)
	}
	for project, testings := range testingProjectMap {
		if allPermissionInfo.IsSystemAdmin {
			setVerbToTestings(respMap, testings, []string{"*"})
			continue
		}
		if projectAuth, ok := allPermissionInfo.ProjectAuthInfo[project]; ok {
			verbList := make([]string, 0)
			if projectAuth.Test.View {
				verbList = append(verbList, VerbGetTest)
			}
			if projectAuth.Test.Edit {
				verbList = append(verbList, VerbEditTest)
			}
			if projectAuth.Test.Delete {
				verbList = append(verbList, VerbDeleteTest)
			}
			if projectAuth.Test.Create {
				verbList = append(verbList, VerbCreateTest)
			}
			if projectAuth.Test.Execute {
				verbList = append(verbList, VerbRunTest)
			}
			setVerbToTestings(respMap, testings, verbList)
			continue
		}
	}
	for _, testing := range respMap {
		testingResp = append(testingResp, testing)
	}

	return testingResp, nil
}

func setVerbToTestings(workflowsNameMap map[string]*TestingOpt, testings []*TestingOpt, verbs []string) {
	for _, testing := range testings {
		testing.Verbs = verbs
		workflowsNameMap[testing.Name] = testing
	}
}
