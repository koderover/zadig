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

package stepcontroller

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type junitReportCtl struct {
	step            *commonmodels.StepTask
	junitReportSpec *step.StepJunitReportSpec
	workflowCtx     *commonmodels.WorkflowTaskCtx
	log             *zap.SugaredLogger
}

func NewJunitReportCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*junitReportCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal junit report spec error: %v", err)
	}
	junitReportSpec := &step.StepJunitReportSpec{}
	if err := yaml.Unmarshal(yamlString, &junitReportSpec); err != nil {
		return nil, fmt.Errorf("unmarshal junit report spec error: %v", err)
	}
	stepTask.Spec = junitReportSpec
	return &junitReportCtl{junitReportSpec: junitReportSpec, log: log, step: stepTask, workflowCtx: workflowCtx}, nil
}

func (s *junitReportCtl) PreRun(ctx context.Context) error {
	if s.junitReportSpec.S3Storage == nil {
		modelS3, err := commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return err
		}
		s.junitReportSpec.S3Storage = modelS3toS3(modelS3)
	}
	s.step.Spec = s.junitReportSpec
	return nil
}

func (s *junitReportCtl) AfterRun(ctx context.Context) error {
	if s.junitReportSpec.TestName == "" {
		return nil
	}
	var testTaskStat *commonmodels.TestTaskStat
	var isNew bool
	testTaskStat, _ = commonrepo.NewTestTaskStatColl().FindTestTaskStat(&commonrepo.TestTaskStatOption{Name: s.junitReportSpec.TestName})
	if testTaskStat == nil {
		isNew = true
		testTaskStat = new(commonmodels.TestTaskStat)
		testTaskStat.Name = s.junitReportSpec.TestName
		testTaskStat.CreateTime = time.Now().Unix()
		testTaskStat.UpdateTime = time.Now().Unix()
	}
	filename, err := util.GenerateTmpFile()
	if err != nil {
		log.Errorf("GenerateTmpFile err:%v", err)
		return err
	}
	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("find defalt s3 error: %v", err)
		return err
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		log.Errorf("NewClient err:%v", err)
		return err
	}
	objectKey := filepath.Join(s.junitReportSpec.S3Storage.Subfolder, s.junitReportSpec.S3DestDir, s.junitReportSpec.FileName)
	err = client.Download(storage.Bucket, objectKey, filename)
	if err != nil {
		log.Errorf("Download junit report err:%v", err)
		return err
	}
	defer os.Remove(filename)

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("get local test result file error: %v", err)
		return err
	}
	testReport := new(commonmodels.TestSuite)
	if err := xml.Unmarshal(b, &testReport); err != nil {
		log.Error("uploadTaskData testSuite unmarshal it report xml error: %v", err)
		return err
	}
	totalCaseNum := testReport.Tests
	if totalCaseNum != 0 {
		testTaskStat.TestCaseNum = totalCaseNum
	}
	testTaskStat.TotalSuccess++
	if isNew {
		_ = commonrepo.NewTestTaskStatColl().Create(testTaskStat)
	} else {
		testTaskStat.UpdateTime = time.Now().Unix()
		_ = commonrepo.NewTestTaskStatColl().Update(testTaskStat)
	}

	duration := 0.0
	for _, cases := range testReport.TestCases {
		duration += cases.Time
	}
	// save the test report information into db for further usage
	err = commonrepo.NewCustomWorkflowTestReportColl().Create(&commonmodels.CustomWorkflowTestReport{
		WorkflowName:     s.junitReportSpec.SourceWorkflow,
		JobName:          s.junitReportSpec.SourceJobKey,
		JobTaskName:      s.junitReportSpec.JobTaskName,
		TaskID:           s.junitReportSpec.TaskID,
		ServiceName:      s.junitReportSpec.ServiceName,
		ServiceModule:    s.junitReportSpec.ServiceModule,
		ZadigTestName:    s.junitReportSpec.TestName,
		ZadigTestProject: s.junitReportSpec.TestProject,
		RetryNum:         s.workflowCtx.RetryNum,
		TestName:         testReport.Name,
		TestCaseNum:      testReport.Tests,
		SuccessCaseNum:   testReport.Successes,
		SkipCaseNum:      testReport.Skips,
		FailedCaseNum:    testReport.Failures,
		ErrorCaseNum:     testReport.Errors,
		TestTime:         math.Round(duration*1000) / 1000,
		TestCases:        testReport.TestCases,
	})

	if err != nil {
		log.Error("save junit test result failed, error: %v", err)
	}

	return nil
}
