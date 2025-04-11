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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	runtimeJobController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	jobcontroller "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/containerlog"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/util"
)

func GetWorkflowV4JobContainerLogs(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	buildJobNamePrefix := jobName
	buildLog, err := getContainerLogFromS3(workflowName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}
	return buildLog, nil
}

func getContainerLogFromS3(pipelineName, filenamePrefix string, taskID int64, log *zap.SugaredLogger) (string, error) {
	fileName := strings.Replace(filenamePrefix, "_", "-", -1)
	fileName += ".log"
	tempFile, _ := util.GenerateTmpFile()
	defer func() {
		_ = os.Remove(tempFile)
	}()

	storage, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("GetContainerLogFromS3 FindDefaultS3 err:%v", err)
		return "", err
	}

	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "log")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "log")
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		log.Errorf("Failed to create s3 client, the error is: %+v", err)
		return "", err
	}
	fullPath := storage.GetObjectPath(fileName)
	err = client.DownloadWithOption(storage.Bucket, fullPath, tempFile, &s3tool.DownloadOption{
		IgnoreNotExistError: true,
		RetryNum:            3,
	})
	if err != nil {
		log.Errorf("GetContainerLogFromS3 Download err:%v", err)
		return "", err
	}

	containerLog, err := ioutil.ReadFile(tempFile)
	if err != nil {
		log.Errorf("GetContainerLogFromS3 Read file err:%v", err)
		return "", err
	}
	return string(containerLog), nil
}

func GetCurrentContainerLogs(podName, containerName, envName, productName string, tailLines int64, log *zap.SugaredLogger) (string, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("Failed to find env %s in project %s, err: %s", envName, productName, err)
		return "", err
	}
	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
	if err != nil {
		log.Errorf("Failed to get kube client, err: %s", err)
		return "", err
	}

	buf := new(bytes.Buffer)
	err = containerlog.GetContainerLogs(env.Namespace, podName, containerName, false, tailLines, buf, clientset)
	if err != nil {
		log.Errorf("Failed to get container logs, err: %s", err)
		return "", err
	}

	return buf.String(), nil
}

func GetScanningContainerLogs(scanID string, taskID int64, log *zap.SugaredLogger) (string, error) {
	workflowName := commonutil.GenScanningWorkflowName(scanID)
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return "", fmt.Errorf("failed to find workflow task for scanning: %s, err: %s", workflowName, err)
	}

	if len(workflowTask.Stages) != 1 {
		log.Errorf("Invalid stage length: stage length for scanning should be 1")
		return "", fmt.Errorf("invalid stage length")
	}

	if len(workflowTask.Stages[0].Jobs) != 1 {
		log.Errorf("Invalid Job length: job length for scanning should be 1")
		return "", fmt.Errorf("invalid job length")
	}

	jobInfo := new(commonmodels.TaskJobInfo)
	if err := commonmodels.IToi(workflowTask.Stages[0].Jobs[0].JobInfo, jobInfo); err != nil {
		return "", fmt.Errorf("convert job info to task job info error: %v", err)
	}

	job := workflowTask.Stages[0].Jobs[0]
	jobName := jobcontroller.GenJobName(workflowTask.WorkflowArgs, job.OriginName, 0)
	if job.OriginName == "" {
		// compatible with old data
		scanning, err := commonrepo.NewScanningColl().GetByID(scanID)
		if err != nil {
			log.Errorf("failed to get scanning from db to get scanning detail, the error is: %s", err)
			return "", err
		}

		name := scanning.Name
		if len(name) >= 32 {
			name = strings.TrimSuffix(scanning.Name[:31], "-")
		}

		jobName = fmt.Sprintf("%s-%s", scanning.Name, name)
	}

	scanningLog, err := getContainerLogFromS3(workflowName, runtimeJobController.GetJobContainerName(jobName), taskID, log)
	if err != nil {
		return "", err
	}

	return scanningLog, nil
}

func GetTestingContainerLogs(testName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	workflowName := commonutil.GenTestingWorkflowName(testName)

	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		log.Errorf("failed to find workflow task for testing: %s, err: %s", testName, err)
		return "", err
	}

	if len(workflowTask.Stages) != 1 {
		log.Errorf("Invalid stage length: stage length for testing should be 1")
		return "", fmt.Errorf("invalid stage length")
	}

	if len(workflowTask.Stages[0].Jobs) != 1 {
		log.Errorf("Invalid Job length: job length for testing should be 1")
		return "", fmt.Errorf("invalid job length")
	}

	job := workflowTask.Stages[0].Jobs[0]
	jobName := jobcontroller.GenJobName(workflowTask.WorkflowArgs, job.OriginName, 0)
	if job.OriginName == "" {
		// compatible with old data
		jobInfo := new(commonmodels.TaskJobInfo)
		if err := commonmodels.IToi(workflowTask.Stages[0].Jobs[0].JobInfo, jobInfo); err != nil {
			return "", fmt.Errorf("convert job info to task job info error: %v", err)
		}
		jobName = strings.ToLower(fmt.Sprintf("%s-%s-%s", jobInfo.JobName, jobInfo.TestingName, jobInfo.RandStr))
	}

	testingLog, err := getContainerLogFromS3(workflowName, runtimeJobController.GetJobContainerName(jobName), taskID, log)
	if err != nil {
		return "", err
	}
	return testingLog, nil
}
