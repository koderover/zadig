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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	s3service "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

func GetBuildJobContainerLogs(pipelineName, serviceName string, taskID int64, log *xlog.Logger) (string, error) {
	buildJobNamePrefix := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineName, taskID, config.TaskBuild, serviceName)
	buildLog, err := getContainerLogFromS3(pipelineName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}

func GetWorkflowBuildJobContainerLogs(pipelineName, serviceName, buildType string, taskID int64, log *xlog.Logger) (string, error) {
	buildJobNamePrefix := fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowType, pipelineName, taskID, buildType, serviceName)
	buildLog, err := getContainerLogFromS3(pipelineName, buildJobNamePrefix, taskID, log)
	if err != nil {
		return "", err
	}

	return buildLog, nil
}

func GetTestJobContainerLogs(pipelineName, serviceName string, taskID int64, log *xlog.Logger) (string, error) {
	taskName := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineName, taskID, config.TaskTestingV2, serviceName)
	return getContainerLogFromS3(pipelineName, taskName, taskID, log)
}

func GetWorkflowTestJobContainerLogs(pipelineName, serviceName, pipelineType string, taskID int64, log *xlog.Logger) (string, error) {
	workflowTypeString := config.WorkflowType
	if pipelineType == string(config.TestType) {
		workflowTypeString = config.TestType
	}

	taskName := fmt.Sprintf("%s-%s-%d-%s-%s", workflowTypeString, pipelineName, taskID, config.TaskTestingV2, serviceName)
	return getContainerLogFromS3(pipelineName, taskName, taskID, log)
}

func getContainerLogFromS3(pipelineName, filenamePrefix string, taskID int64, log *xlog.Logger) (string, error) {
	fileName := strings.Replace(strings.ToLower(filenamePrefix), "_", "-", -1)
	tempFile, _ := util.GenerateTmpFile()
	defer func() {
		_ = os.Remove(tempFile)
	}()
	if storage, err := s3service.FindDefaultS3(); err == nil {
		if storage.Subfolder != "" {
			storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "log")
		} else {
			storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "log")
		}
		if files, err := s3service.ListFiles(storage, fileName, false); err == nil && len(files) > 0 {
			if err = s3service.Download(context.Background(), storage, files[0], tempFile); err == nil {
				var log []byte
				log, err = ioutil.ReadFile(tempFile)
				if err == nil {
					return string(log), nil
				}
			} else {
				log.Errorf("GetContainerLogFromS3 Download err:%v", err)
			}
		} else if err != nil {
			log.Errorf("GetContainerLogFromS3 ListFiles err:%v", err)
		}
	} else {
		log.Errorf("GetContainerLogFromS3 FindDefaultS3 err:%v", err)
	}

	log.Errorf("failed to find file with path %s", tempFile)
	err := fmt.Errorf("log file not found")
	return "", err
}
