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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/containerlog"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

func GetBuildJobContainerLogs(pipelineName, serviceName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	buildJobNamePrefix := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineName, taskID, config.TaskBuild, serviceName)
	buildLog, err := getContainerLogFromS3(pipelineName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}

func GetWorkflowBuildJobContainerLogs(pipelineName, serviceName, buildType string, taskID int64, log *zap.SugaredLogger) (string, error) {
	buildJobNamePrefix := fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowType, pipelineName, taskID, buildType, serviceName)
	buildLog, err := getContainerLogFromS3(pipelineName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}

func GetWorkflowV4JobContainerLogs(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	buildJobNamePrefix := jobName
	buildLog, err := getContainerLogFromS3(workflowName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}
	return buildLog, nil
}

func GetTestJobContainerLogs(pipelineName, serviceName string, taskID int64, log *zap.SugaredLogger) (string, error) {
	taskName := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineName, taskID, config.TaskTestingV2, serviceName)
	return getContainerLogFromS3(pipelineName, taskName, taskID, log)
}

func GetWorkflowTestJobContainerLogs(pipelineName, serviceName, pipelineType string, taskID int64, log *zap.SugaredLogger) (string, error) {
	workflowTypeString := config.WorkflowType
	if pipelineType == string(config.TestType) {
		workflowTypeString = config.TestType
	}

	taskName := fmt.Sprintf("%s-%s-%d-%s-%s", workflowTypeString, pipelineName, taskID, config.TaskTestingV2, serviceName)
	return getContainerLogFromS3(pipelineName, taskName, taskID, log)
}

func getContainerLogFromS3(pipelineName, filenamePrefix string, taskID int64, log *zap.SugaredLogger) (string, error) {
	fileName := strings.Replace(strings.ToLower(filenamePrefix), "_", "-", -1)
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
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, forcedPathStyle)
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
	clientset, err := kube.GetClientset(env.ClusterID)
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

func GetWorkflowBuildV3JobContainerLogs(workflowName, buildType string, taskID int64, log *zap.SugaredLogger) (string, error) {
	buildJobNamePrefix := fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowTypeV3, workflowName, taskID, buildType, fmt.Sprintf("%s-job", workflowName))
	buildLog, err := getContainerLogFromS3(workflowName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}

func GetScanningContainerLogs(scanID string, taskID int64, log *zap.SugaredLogger) (string, error) {
	scanning, err := commonrepo.NewScanningColl().GetByID(scanID)
	if err != nil {
		log.Errorf("failed to get scanning from db to get scanning detail, the error is: %s", err)
		return "", err
	}
	scanningName := fmt.Sprintf("%s-%s-%s", scanning.Name, scanID, "scanning-job")
	scanningLogFilePrefix := fmt.Sprintf("%s-%s-%d-%s", config.ScanningType, scanningName, taskID, config.ScanningType)
	buildLog, err := getContainerLogFromS3(scanningName, scanningLogFilePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}
