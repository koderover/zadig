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
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	jenkins "github.com/koderover/gojenkins"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	vmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/vm/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/containerlog"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/watcher"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	timeout = 5 * time.Minute
)

type GetContainerOptions struct {
	Namespace     string
	PipelineName  string
	SubTask       string
	JobName       string
	JobType       string
	TailLines     int64
	TaskID        int64
	PipelineType  string
	ServiceName   string
	ServiceModule string
	TestName      string
	EnvName       string
	ProductName   string
	ClusterID     string
}

type GetVMJobLogOptions struct {
	Infrastructure string
	ProjectKey     string
	WorkflowKey    string
	TaskID         int64
	JobName        string
}

func ContainerLogStream(ctx context.Context, streamChan chan interface{}, envName, productName, podName, containerName string, follow bool, tailLines int64, log *zap.SugaredLogger) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("kubeCli.GetContainerLogStream error: %v", err)
		return
	}
	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to find ns and kubeClient: %v", err)
		return
	}
	containerLogStream(ctx, streamChan, productInfo.Namespace, podName, containerName, follow, tailLines, clientset, log)
}

func containerLogStream(ctx context.Context, streamChan chan interface{}, namespace, podName, containerName string, follow bool, tailLines int64, client *kubernetes.Clientset, log *zap.SugaredLogger) {
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
				if strings.ContainsRune(line, '\r') {
					segments := strings.Split(line, "\r")
					for _, segment := range segments {
						segment = segment + string('\r')
						if len(segment) > 0 {
							streamChan <- segment
						}
					}
				} else {
					line = strings.TrimSpace(line)
					streamChan <- line
				}
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

func parseServiceName(fullServiceName, serviceModule string) (string, string) {
	// when service module is passed, use the passed value
	// otherwise we fall back to the old logic
	if len(serviceModule) > 0 {
		return strings.TrimPrefix(fullServiceName, serviceModule+"_"), serviceModule
	}
	var serviceName string
	serviceNames := strings.Split(fullServiceName, "_")
	switch len(serviceNames) {
	case 1:
		serviceModule = serviceNames[0]
	case 2:
		// Note: Starting from V1.10.0, this field will be in the format of `ServiceModule_ServiceName`.
		serviceModule = serviceNames[0]
		serviceName = serviceNames[1]
	}
	return serviceName, serviceModule
}

func WorkflowTaskV4ContainerLogStream(ctx context.Context, streamChan chan interface{}, options *GetContainerOptions, log *zap.SugaredLogger) {
	if options == nil {
		return
	}
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(options.PipelineName, options.TaskID)
	if err != nil {
		log.Errorf("Failed to find workflow %s taskID %s: %v", options.PipelineName, options.TaskID, err)
		return
	}
	var vmJobOptions *GetVMJobLogOptions

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if jobcontroller.GetJobContainerName(job.Name) != options.SubTask {
				continue
			}
			options.JobName = job.K8sJobName
			options.JobType = job.JobType
			switch job.JobType {
			case string(config.JobZadigBuild):
				fallthrough
			case string(config.JobZadigVMDeploy):
				fallthrough
			case string(config.JobFreestyle):
				fallthrough
			case string(config.JobZadigTesting):
				fallthrough
			case string(config.JobZadigScanning):
				fallthrough
			case string(config.JobZadigDistributeImage):
				fallthrough
			case string(config.JobBuild):
				jobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
					log.Errorf("Failed to parse job spec: %v", err)
					return
				}

				if job.Infrastructure == setting.JobVMInfrastructure {
					vmJobOptions = &GetVMJobLogOptions{
						Infrastructure: job.Infrastructure,
						ProjectKey:     task.ProjectName,
						WorkflowKey:    task.WorkflowName,
						TaskID:         task.TaskID,
						JobName:        job.Name,
					}
				} else {
					options.ClusterID = jobSpec.Properties.ClusterID
				}
			case string(config.JobPlugin):
				jobSpec := &commonmodels.JobTaskPluginSpec{}
				if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
					log.Errorf("Failed to parse job spec: %v", err)
					return
				}
				options.ClusterID = jobSpec.Properties.ClusterID
			default:
				log.Errorf("get real-time log error, unsupported job type %s", job.JobType)
				return
			}
			if options.ClusterID == "" {
				options.ClusterID = setting.LocalClusterID
			}
			switch options.ClusterID {
			case setting.LocalClusterID:
				options.Namespace = config.Namespace()
			default:
				options.Namespace = setting.AttachedClusterNamespace
			}
			break
		}
	}

	if vmJobOptions != nil && vmJobOptions.Infrastructure == setting.JobVMInfrastructure {
		waitVmAndGetLog(ctx, streamChan, vmJobOptions, log)
	} else {
		selector := getWorkflowSelector(options)
		waitAndGetLog(ctx, streamChan, selector, options, log)
	}
}

func getTestName(serviceName string) string {
	testName := strings.TrimRight(serviceName, "-job")
	return testName
}

func waitAndGetLog(ctx context.Context, streamChan chan interface{}, selector labels.Selector, options *GetContainerOptions, log *zap.SugaredLogger) {
	PodCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debugf("Waiting until pod is running before establishing the stream. labelSelector: %+v, clusterId: %s, namespace: %s", selector, options.ClusterID, options.Namespace)
	clientSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(options.ClusterID)
	if err != nil {
		log.Errorf("GetContainerLogs, get client set error: %s", err)
		return
	}

	err = watcher.WaitUntilPodRunning(PodCtx, options.Namespace, selector, clientSet)
	if err != nil {
		log.Errorf("GetContainerLogs, wait pod running error: %s", err)
		return
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(options.ClusterID)
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

func waitVmAndGetLog(ctx context.Context, streamChan chan interface{}, options *GetVMJobLogOptions, log *zap.SugaredLogger) {
	job, err := vmmongodb.NewVMJobColl().FindByOpts(vmmongodb.VMJobFindOption{
		ProjectName:  options.ProjectKey,
		WorkflowName: options.WorkflowKey,
		TaskID:       options.TaskID,
		JobName:      options.JobName,
	})
	if err != nil {
		log.Errorf("get vm job error: %v", err)
		return
	}

	if job.LogFile == "" {
		log.Errorf("vm job log file is empty")
		return
	}

	if job.Status != string(config.StatusRunning) {
		log.Errorf("vm job not running")
		return
	}

	out, err := os.OpenFile(job.LogFile, os.O_APPEND|os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		log.Errorf("open vm job log file error: %v", err)
		return
	}
	defer func() {
		err := out.Close()
		if err != nil {
			log.Errorf("Failed to close vm job log file, error: %v", err)
		}
	}()

	buf := bufio.NewReader(out)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Connection is closed, vm log stream stopped")
			return
		default:
			if !vmservice.VMJobStatus.Exists(job.ID.Hex()) {
				err := ReadFromFileAndWriteToStreamChan(buf, streamChan)
				if err != nil && err != io.EOF {
					log.Errorf("scan vm log stream error: %v", err)
					return
				}
				log.Infof("job cache existed vm job log stream stopped")
				return
			}

			err := ReadFromFileAndWriteToStreamChan(buf, streamChan)
			if err != nil && err != io.EOF {
				log.Errorf("scan vm log stream error: %v", err)
				return
			}

			time.Sleep(1000 * time.Millisecond)
		}
	}
}

func ReadFromFileAndWriteToStreamChan(buf *bufio.Reader, streamChan chan interface{}) error {
	for {
		line, err := buf.ReadString('\n')
		if err == nil {
			if strings.ContainsRune(line, '\r') {
				segments := strings.Split(line, "\r")
				for _, segment := range segments {
					segment = segment + string('\r')
					if len(segment) > 0 {
						streamChan <- segment
					}
				}
			} else {
				if len(line) > 0 {
					streamChan <- line
				}
			}
			continue
		}
		return err
	}
}

func getWorkflowSelector(options *GetContainerOptions) labels.Selector {
	retMap := map[string]string{
		setting.JobLabelSTypeKey: strings.Replace(options.JobType, "_", "-", -1),
		setting.JobLabelNameKey:  strings.Replace(options.JobName, "_", "-", -1),
	}
	// no need to add labels with empty value to a job
	for k, v := range retMap {
		if len(v) == 0 {
			delete(retMap, k)
		}
	}
	return labels.Set(retMap).AsSelector()
}

func JenkinsJobLogStream(ctx context.Context, jenkinsID, jobName string, jobID int64, streamChan chan interface{}) {
	log := log.SugaredLogger().With("func", "JenkinsJobLogStream")
	info, err := commonrepo.NewCICDToolColl().Get(jenkinsID)
	if err != nil {
		log.Errorf("Failed to get jenkins integration info, err: %s", err)
		return
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: transport}
	jenkinsClient, err := jenkins.CreateJenkins(client, info.URL, info.Username, info.Password).Init(context.TODO())

	if err != nil {
		log.Errorf("failed to create jenkins client for server, the error is: %s", err)
		return
	}

	build, err := jenkinsClient.GetBuild(context.Background(), jobName, jobID)
	if err != nil {
		log.Errorf("failed to get build info from jenkins, error is: %s", err)
		return
	}

	var offset int64 = 0
	for {
		select {
		case <-ctx.Done():
			log.Infof("context done, stop streaming")
			return
		default:
		}
		time.Sleep(1000 * time.Millisecond)
		build.Poll(context.TODO())
		consoleOutput, err := build.GetConsoleOutputFromIndex(context.TODO(), offset)
		if err != nil {
			log.Warnf("failed to get logs from jenkins job, error: %s", err)
			return
		}
		for _, str := range strings.Split(consoleOutput.Content, "\r\n") {
			streamChan <- str
		}
		offset += consoleOutput.Offset
		if !build.IsRunning(context.TODO()) {
			return
		}
	}
}
