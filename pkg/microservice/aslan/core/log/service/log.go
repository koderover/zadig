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
	"context"
	"fmt"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	//"golang.org/x/net/context/ctxhttp"
	"io"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"os"
	//"os/exec"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
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
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure)
	if err != nil {
		log.Errorf("Failed to create s3 client, the error is: %+v", err)
		return "", err
	}
	objectPrefix := storage.GetObjectPath(fileName)
	fileList, err := client.ListFiles(storage.Endpoint, objectPrefix, false)
	if err != nil {
		log.Errorf("GetContainerLogFromS3 ListFiles err:%v", err)
		return "", err
	}
	if len(fileList) == 0 {
		return "", nil
	}
	objectKey := storage.GetObjectPath(fileList[0])
	err = client.Download(storage.Bucket, objectKey, tempFile)
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

func GetPodLogByHttp(podName, containerName, envName, productName string, tail int64, log *zap.SugaredLogger) (string, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("kubeCli.GetContainerLogStream error: %v", err)
		return "", err
	}
	clientSet, err := kube.GetClientset(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to find ns and kubeClient: %v", err)
		return "", err
	}

	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tail,
	}
	ctx := context.TODO()
	req := clientSet.CoreV1().Pods(productInfo.Namespace).GetLogs(podName, logOptions)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		log.Errorf("req.Stream error: %v,productInfo.Namespace:%v,podName:%v,containerName:%v", err,
			productInfo.Namespace, podName, containerName)
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		log.Errorf("io.Copy error: %v", err)
		return "", err
	}
	str := buf.String()

	return str, nil

	/*cmdStr := ""
	if len(containerName) > 0 {
		cmdStr = fmt.Sprintf("kubectl logs  %v -c %v --tail %d", podName, containerName, tail)
	} else {
		cmdStr = fmt.Sprintf("kubectl logs %v --tail %d", podName, tail)

	}
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.Output()
	if err != nil {
		//fmt.Printf("exec.Command error:%v,cmd:%v \n",err,cmdStr)
		log.Errorf("exec.Command error:%v", cmdStr)
		return "", err
	} else {
		return string(output), nil
	}*/
}
