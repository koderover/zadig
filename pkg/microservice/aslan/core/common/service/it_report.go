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
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/setting"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

func GetLocalTestSuite(pipelineName, serviceName, testType string, taskID int64, testName string, typeString config.PipelineType, log *zap.SugaredLogger) (*commonmodels.TestReport, error) {
	testReport := new(commonmodels.TestReport)
	// 文件名称都是小写存储的
	// testName param is deperecated here.
	// TODO: clear frontend & backend code deperecated testName(actually it's test subtask jobName)
	// testName should be: pipelineName-taskId-taskType-serviceName
	pipelineTask, err := mongodb.NewTaskColl().Find(taskID, pipelineName, typeString)
	if err != nil {
		return testReport, fmt.Errorf("cannot find pipeline task, pipeline name: %s, task id: %d", pipelineName, taskID)
	}

	testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
		config.SingleType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, pipelineTask.ServiceName)), "_", "-", -1)
	if typeString == config.WorkflowType {
		testJobName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	} else if typeString == config.TestType {
		testJobName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.TestType, pipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	}
	//resultPath := path.Join(s.Config.NFS.Path, pipelineName, "dist", fmt.Sprintf("%d", taskID))
	//resultPath := path.Join(pipelineName, "dist", fmt.Sprintf("%d", taskID))
	//testResultFile := path.Join(resultPath, testJobName)

	if testType == "undefined" {
		testType = setting.FunctionTest
	}
	//s3Service := s3.S3Service{}
	var s3Storage *s3.S3
	var filename string
	if s3Storage, err = s3.FindDefaultS3(); err == nil {
		filename, err = util.GenerateTmpFile()
		defer func() {
			_ = os.Remove(filename)
		}()
		if err != nil {
			log.Errorf("GetLocalTestSuite GenerateTmpFile err:%v", err)
		}
		objectKey := s3Storage.GetObjectPath(fmt.Sprintf("%s/%d/%s/%s", pipelineName, taskID, "test", testJobName))
		client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, s3Storage.Provider)
		if err != nil {
			log.Errorf("Failed to create s3 client for download, error: %+v", err)
			return testReport, fmt.Errorf("failed to create s3 client for download, error: %+v", err)
		}
		if err = client.Download(s3Storage.Bucket, objectKey, filename); err != nil {
			log.Errorf("GetLocalTestSuite s3 Download err:%v", err)
			return testReport, fmt.Errorf("getLocalTestSuite s3 Download err: %v", err)
		}
	} else {
		log.Errorf("GetLocalTestSuite FindDefaultS3 err:%v", err)
		return testReport, fmt.Errorf("GetLocalTestSuite FindDefaultS3 err: %v", err)
	}
	if testType == setting.FunctionTest {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			msg := fmt.Sprintf("GetLocalTestSuite get local test result file error: %v", err)
			log.Error(msg)
			return testReport, errors.New(msg)
		}
		if err := xml.Unmarshal(b, &testReport.FunctionTestSuite); err != nil {
			msg := fmt.Sprintf("GetLocalTestSuite unmarshal it report xml error: %v", err)
			log.Error(msg)
			return testReport, errors.New(msg)
		}
		return testReport, nil
	}
	csvFile, err := os.Open(filename)
	if err != nil {
		msg := fmt.Sprintf("GetLocalTestSuite get local test result file error: %v", err)
		return testReport, errors.New(msg)
	}
	defer csvFile.Close()

	csvReader := csv.NewReader(csvFile)
	row, err := csvReader.Read()
	if len(row) != 11 {
		msg := "GetLocalTestSuite csv file type match error"
		return testReport, errors.New(msg)
	}
	if err != nil {
		msg := fmt.Sprintf("GetLocalTestSuite read csv first row error: %v", err)
		return testReport, errors.New(msg)
	}
	rows, err := csvReader.ReadAll()
	if err != nil {
		msg := fmt.Sprintf("GetLocalTestSuite read csv readAll error: %v", err)
		return testReport, errors.New(msg)
	}
	performanceTestSuites := make([]*commonmodels.PerformanceTestSuite, 0)
	for _, row := range rows {
		performanceTestSuite := new(commonmodels.PerformanceTestSuite)
		performanceTestSuite.Label = row[0]
		performanceTestSuite.Samples = row[1]
		performanceTestSuite.Average = row[2]
		performanceTestSuite.Min = row[3]
		performanceTestSuite.Max = row[4]
		performanceTestSuite.Line = row[5]
		performanceTestSuite.StdDev = row[6]
		performanceTestSuite.Error = row[7]
		performanceTestSuite.Throughput = row[8]
		performanceTestSuite.ReceivedKb = row[9]
		performanceTestSuite.AvgByte = row[10]

		performanceTestSuites = append(performanceTestSuites, performanceTestSuite)
	}
	testReport.PerformanceTestSuites = performanceTestSuites
	return testReport, nil
}

func GetWorkflowV4LocalTestSuite(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) (*commonmodels.TestReport, error) {
	testReport := new(commonmodels.TestReport)
	workflowTask, err := mongodb.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return testReport, fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != string(config.JobZadigTesting) {
				return testReport, fmt.Errorf("job: %s was not a testing job", jobName)
			}
			jobTask = job
		}
	}
	if jobTask == nil {
		return testReport, fmt.Errorf("cannot find job task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		return testReport, fmt.Errorf("unmashal job spec error: %v", err)
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if step.Name != config.TestJobJunitReportStepName {
			continue
		}
		if step.StepType != config.StepJunitReport {
			return testReport, fmt.Errorf("step: %s was not a junit report step", step.Name)
		}
		stepTask = step
	}
	if stepTask == nil {
		return testReport, fmt.Errorf("cannot find step task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	stepSpec := &step.StepJunitReportSpec{}
	if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
		return testReport, fmt.Errorf("unmashal step spec error: %v", err)
	}

	var s3Storage *s3.S3
	var filename string
	if s3Storage, err = s3.FindDefaultS3(); err == nil {
		filename, err = util.GenerateTmpFile()
		defer func() {
			_ = os.Remove(filename)
		}()
		if err != nil {
			log.Errorf("GetLocalTestSuite GenerateTmpFile err:%v", err)
		}

		if len(stepSpec.S3Storage.Subfolder) > 0 {
			stepSpec.S3DestDir = strings.TrimLeft(path.Join(stepSpec.S3Storage.Subfolder, stepSpec.S3DestDir), "/")
		}
		objectKey := filepath.Join(stepSpec.S3DestDir, stepSpec.FileName)
		client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, s3Storage.Provider)
		if err != nil {
			log.Errorf("Failed to create s3 client for download, error: %+v", err)
			return testReport, fmt.Errorf("failed to create s3 client for download, error: %+v", err)
		}
		if err = client.Download(s3Storage.Bucket, objectKey, filename); err != nil {
			if strings.Contains(err.Error(), "NoSuchKey") {
				return testReport, fmt.Errorf("getLocalTestSuite s3 Download err: %v", err)
			}
			log.Errorf("GetLocalTestSuite s3 Download err:%v", err)
			return testReport, fmt.Errorf("getLocalTestSuite s3 Download err: %v", err)
		}
	} else {
		log.Errorf("GetLocalTestSuite FindDefaultS3 err:%v", err)
		return testReport, fmt.Errorf("GetLocalTestSuite FindDefaultS3 err: %v", err)
	}

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		msg := fmt.Sprintf("GetLocalTestSuite get local test result file error: %v", err)
		log.Error(msg)
		return testReport, errors.New(msg)
	}
	if err := xml.Unmarshal(b, &testReport.FunctionTestSuite); err != nil {
		msg := fmt.Sprintf("GetLocalTestSuite unmarshal it report xml error: %v", err)
		log.Error(msg)
		return testReport, errors.New(msg)
	}
	return testReport, nil
}
