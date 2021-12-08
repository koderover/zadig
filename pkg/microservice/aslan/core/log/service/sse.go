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
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/containerlog"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/watcher"
)

const (
	timeout = 5 * time.Minute
)

type GetContainerOptions struct {
	Namespace    string
	PipelineName string
	SubTask      string
	TailLines    int64
	TaskID       int64
	PipelineType string
	ServiceName  string
	TestName     string
	EnvName      string
	ProductName  string
	ClusterID    string
}

func ContainerLogStream(ctx context.Context, streamChan chan interface{}, envName, productName, podName, containerName string, follow bool, tailLines int64, log *zap.SugaredLogger) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("kubeCli.GetContainerLogStream error: %v", err)
		return
	}
	clientset, err := kube.GetClientset(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to find ns and kubeClient: %v", err)
		return
	}
	containerLogStream(ctx, streamChan, productInfo.Namespace, podName, containerName, follow, tailLines, clientset, log)
}

func containerLogStream(ctx context.Context, streamChan chan interface{}, namespace, podName, containerName string, follow bool, tailLines int64, client kubernetes.Interface, log *zap.SugaredLogger) {
	log.Infof("[GetContainerLogsSSE] Get container log of pod %s", podName)

	out, err := containerlog.GetContainerLogStream(ctx, namespace, podName, containerName, follow, tailLines, client)
	if err != nil {
		log.Errorf("kubeCli.GetContainerLogStream error: %v", err)
		return
	}
	defer func() {
		err := out.Close()
		if err != nil {
			log.Errorf("Failed to close container log stream, error: %v", err)
		}
	}()

	buf := bufio.NewReader(out)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Connection is closed, container log stream stopped")
			return
		default:
			line, err := buf.ReadString('\n')
			if err == nil {
				line = strings.TrimSpace(line)
				streamChan <- line
			}
			if err == io.EOF {
				line = strings.TrimSpace(line)
				if len(line) > 0 {
					streamChan <- line
				}
				log.Infof("No more input is available, container log stream stopped")
				return
			}

			if err != nil {
				log.Errorf("scan container log stream error: %v", err)
				return
			}
		}
	}
}

func TaskContainerLogStream(ctx context.Context, streamChan chan interface{}, options *GetContainerOptions, log *zap.SugaredLogger) {
	if options == nil {
		return
	}
	log.Debugf("Start to get task container log.")
	// Cloud host scenario reads real-time logs from the environment, so pipelineName is empty
	if options.EnvName != "" && options.ProductName != "" && options.PipelineName == "" {
		//修改pipelineName，判断pipelineName是否为空，为空代表是来自环境里面请求，不为空代表是来自工作流任务的请求
		options.PipelineName = fmt.Sprintf("%s-%s-%s", options.ServiceName, options.EnvName, "job")
		if taskObj, err := commonrepo.NewTaskColl().FindTask(options.PipelineName, config.ServiceType); err == nil {
			options.TaskID = taskObj.TaskID
		}
		// Need to get build info based on the project name and service component name, then get clusterID and namespace
	} else if options.ProductName != "" {
		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{
			ProductName: options.ProductName,
			Targets:     []string{options.ServiceName},
		})
		if err != nil {
			// Maybe this service is a shared service
			build, err = commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{
				Targets: []string{options.ServiceName},
			})
		}
		if build != nil && build.PreBuild != nil {
			options.ClusterID = build.PreBuild.ClusterID
			options.Namespace = build.PreBuild.Namespace
		}
	}

	if options.SubTask == "" {
		options.SubTask = string(config.TaskBuild)
	}
	selector := getPipelineSelector(options)
	waitAndGetLog(ctx, streamChan, selector, options, log)
}

func TestJobContainerLogStream(ctx context.Context, streamChan chan interface{}, options *GetContainerOptions, log *zap.SugaredLogger) {
	options.SubTask = string(config.TaskTestingV2)
	selector := getPipelineSelector(options)
	// get cluster ID
	testing, _ := commonrepo.NewTestingColl().Find(getTestName(options.ServiceName), "")
	if testing != nil && testing.PreTest != nil {
		options.ClusterID = testing.PreTest.ClusterID
		options.Namespace = testing.PreTest.Namespace
	}

	waitAndGetLog(ctx, streamChan, selector, options, log)
}

func getTestName(serviceName string) string {
	testName := strings.TrimRight(serviceName, "-job")
	return testName
}

func waitAndGetLog(ctx context.Context, streamChan chan interface{}, selector labels.Selector, options *GetContainerOptions, log *zap.SugaredLogger) {
	PodCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debugf("Waiting until pod is running before establishing the stream.")
	clientSet, err := kubeclient.GetClientset(config.HubServerAddress(), options.ClusterID)
	if err != nil {
		log.Errorf("GetContainerLogs, get client set error: %s", err)
		return
	}
	err = watcher.WaitUntilPodRunning(PodCtx, options.Namespace, selector, clientSet)
	if err != nil {
		log.Errorf("GetContainerLogs, wait pod running error: %s", err)
		return
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), options.ClusterID)
	if err != nil {
		log.Errorf("GetContainerLogs, get kube client error: %s", err)
		return
	}
	pods, err := getter.ListPods(options.Namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("GetContainerLogs, get pod error: %+v", err)
		return
	}

	log.Debugf("Found %d running pods", len(pods))

	if len(pods) > 0 {
		containerLogStream(
			ctx, streamChan,
			options.Namespace,
			pods[0].Name, options.SubTask,
			true,
			options.TailLines,
			clientSet,
			log,
		)
	}
}

func getPipelineSelector(options *GetContainerOptions) labels.Selector {
	ret := labels.Set{}
	pipelineWithTaskID := fmt.Sprintf("%s-%d", strings.ToLower(options.PipelineName), options.TaskID)

	//适配之前的docker_build下划线
	options.SubTask = strings.Replace(options.SubTask, "_", "-", 1)

	ret[setting.TaskLabel] = pipelineWithTaskID
	ret[setting.TypeLabel] = options.SubTask

	if options.ServiceName != "" {
		ret[setting.ServiceLabel] = strings.ToLower(options.ServiceName)
	}

	if options.PipelineType != "" {
		ret[setting.PipelineTypeLable] = options.PipelineType
	}

	return ret.AsSelector()
}
