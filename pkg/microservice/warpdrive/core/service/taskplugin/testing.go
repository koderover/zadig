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

package taskplugin

import (
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/label"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	commontypes "github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

func InitializeTestTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &TestPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
		apiReader:  krkubeclient.APIReader(),
	}
}

// TestPlugin name should be compatible with task type
type TestPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	clientset     kubernetes.Interface
	restConfig    *rest.Config
	apiReader     client.Reader
	Task          *task.Testing
	Log           *zap.SugaredLogger
	Timeout       <-chan time.Time
}

func (p *TestPlugin) SetAckFunc(func()) {
}

const (
	TestingV2TaskTimeout = 60 * 60 // 60 minutes
)

func (p *TestPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.FileName = filename
	p.Log = xl
}

func (p *TestPlugin) Type() config.TaskType {
	return p.Name
}

func (p *TestPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

func (p *TestPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

func (p *TestPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = TestingV2TaskTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

func (p *TestPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	if p.Task.CacheEnable && !pipelineTask.ConfigPayload.ResetCache {
		pipelineCtx.CacheEnable = true
		pipelineCtx.Cache = p.Task.Cache
		pipelineCtx.CacheDirType = p.Task.CacheDirType
		pipelineCtx.CacheUserDir = p.Task.CacheUserDir
	} else {
		pipelineCtx.CacheEnable = false
	}

	// TODO: Since the namespace field has been used continuously since v1.10.0, the processing logic related to namespace needs to
	// be deleted in v1.11.0.
	switch p.Task.ClusterID {
	case setting.LocalClusterID:
		p.KubeNamespace = zadigconfig.Namespace()
	default:
		p.KubeNamespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, apiReader, err := GetK8sClients(pipelineTask.ConfigPayload.HubServerAddr, p.Task.ClusterID)
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}

		p.kubeClient = crClient
		p.clientset = clientset
		p.restConfig = restConfig
		p.apiReader = apiReader
	}

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)
	if p.Task.ClusterID != "" && p.Task.ClusterID != setting.LocalClusterID {
		if strings.Contains(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace) {
			// replace namespace only
			dockerHost = strings.Replace(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace, KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(pipelineTask.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if p.Task.ClusterID == "" || p.Task.ClusterID == setting.LocalClusterID {
		if !strings.Contains(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace) {
			// add namespace
			dockerHost = strings.Replace(pipelineTask.DockerHost, replaceDindServer, replaceDindServer+"."+pipelineTask.ConfigPayload.Build.KubeNamespace, 1)
		}
	}
	pipelineCtx.DockerHost = dockerHost
	// Reset error message.
	p.Task.Error = ""
	var linkedNamespace string
	var envName string

	workflowName := pipelineTask.PipelineName
	if pipelineTask.Type == config.SingleType {
		linkedNamespace = pipelineTask.TaskArgs.Test.Namespace
	} else if pipelineTask.Type == config.WorkflowType {
		product := &types.Product{EnvName: pipelineTask.WorkflowArgs.Namespace, ProductName: pipelineTask.WorkflowArgs.ProductTmplName}
		linkedNamespace = product.ProductName + "-env-" + product.EnvName
		envName = pipelineTask.WorkflowArgs.Namespace

		workflowName = pipelineTask.WorkflowArgs.WorkflowName
	}
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "WORKFLOW", Value: workflowName})

	namespaceEnvVar := &task.KeyVal{Key: "DEPLOY_ENV", Value: p.KubeNamespace, IsCredential: false}
	linkedNamespaceEnvVar := &task.KeyVal{Key: "LINKED_ENV", Value: linkedNamespace, IsCredential: false}
	envNameEnvVar := &task.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false}

	// 如果是restart的job, 路径删除原来的workspace地址, 兼容老版本
	prefix := pipelineCtx.Workspace + string(os.PathSeparator)
	if strings.HasPrefix(p.Task.JobCtx.TestResultPath, prefix) {
		p.Task.JobCtx.TestResultPath = p.Task.JobCtx.TestResultPath[len(prefix):len(p.Task.JobCtx.TestResultPath)]
	}

	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, namespaceEnvVar)
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, linkedNamespaceEnvVar)
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envNameEnvVar)
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "SERVICE_MODULE", Value: pipelineTask.ServiceName})
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "PROJECT", Value: pipelineTask.ProductName})

	var testReportFile string // html 测试报告
	fileName := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, pipelineTask.ServiceName)
	// 测试结果保存名称,两种模式需要区分开
	if pipelineTask.Type == config.WorkflowType {
		fileName = fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)
		testReportFile = fmt.Sprintf("%s-%s-%d-%s-%s-html", config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, p.Task.TestModuleName)
	} else if pipelineTask.Type == config.TestType {
		fileName = fmt.Sprintf("%s-%s-%d-%s-%s", config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)
		testReportFile = fmt.Sprintf("%s-%s-%d-%s-%s-html", config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, p.Task.TestModuleName)
	}

	fileName = strings.Replace(strings.ToLower(fileName), "_", "-", -1)
	testReportFile = strings.Replace(strings.ToLower(testReportFile), "_", "-", -1)

	// Since we allow users to use custom environment variables, variable resolution is required.
	if pipelineCtx.CacheEnable && pipelineCtx.Cache.MediumType == commontypes.NFSMedium &&
		pipelineCtx.CacheDirType == commontypes.UserDefinedCacheDir {
		pipelineCtx.CacheUserDir = p.renderEnv(pipelineCtx.CacheUserDir)
		pipelineCtx.Cache.NFSProperties.Subpath = p.renderEnv(pipelineCtx.Cache.NFSProperties.Subpath)
	}

	jobCtx := JobCtxBuilder{
		JobName:        p.JobName,
		PipelineCtx:    pipelineCtx,
		ArchiveFile:    fileName,
		TestReportFile: testReportFile,
		JobCtx:         p.Task.JobCtx,
		Installs:       p.Task.InstallCtx,
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx.BuildReaperContext(pipelineTask, serviceName))
	if err != nil {
		msg := fmt.Sprintf("cannot reaper.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		p.Log.Error(err)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = err.Error()
		return
	}

	if err := createJobConfigMap(p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	jobImage := getReaperImage(pipelineTask.ConfigPayload.Release.ReaperImage, p.Task.BuildOS, p.Task.ImageFrom)

	p.Task.Registries = getMatchedRegistries(jobImage, p.Task.Registries)
	// search namespace should also include desired namespace
	job, err := buildJobWithLinkedNs(
		p.Type(), jobImage, p.JobName, serviceName, p.Task.ClusterID, pipelineTask.ConfigPayload.Test.KubeNamespace, p.Task.ResReq, p.Task.ResReqSpec, pipelineCtx, pipelineTask, p.Task.Registries,
	)

	job.Namespace = p.KubeNamespace

	if err != nil {
		msg := fmt.Sprintf("create testing job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete testing job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	// 将集成到KodeRover的私有镜像仓库的访问权限设置到namespace中
	if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
		p.Log.Errorf("create secret error: %v", err)
	}
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create testing job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}
	p.Timeout = time.After(time.Duration(p.TaskTimeout()) * time.Second)
	p.Task.TaskStatus, err = waitJobReady(ctx, p.KubeNamespace, p.JobName, p.kubeClient, p.apiReader, p.Timeout, p.Log)
	if err != nil {
		p.Task.Error = err.Error()
	}
}

func (p *TestPlugin) Wait(ctx context.Context) {
	status, err := waitJobEndWithFile(ctx, p.TaskTimeout(), p.Timeout, p.KubeNamespace, p.JobName, true, p.kubeClient, p.clientset, p.restConfig, p.Log)
	p.SetStatus(status)
	if err != nil {
		p.Task.Error = err.Error()
	}
}

func (p *TestPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	pipelineName := pipelineTask.PipelineName
	pipelineTaskID := pipelineTask.TaskID

	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// Clean up tasks that user canceled or timed out.
	defer func() {
		go func() {
			if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
			}

			if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
			}
		}()
	}()

	err := saveContainerLog(pipelineTask, p.KubeNamespace, p.Task.ClusterID, p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		if p.Task.Error == "" {
			p.Task.Error = err.Error()
		}
		return
	}
	p.Task.LogFile = p.JobName

	fileName := fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, pipelineTask.ServiceName)

	// 测试结果保存名称,两种模式需要区分开
	if pipelineTask.Type == config.WorkflowType {
		fileName = fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)
	} else if pipelineTask.Type == config.TestType {
		fileName = fmt.Sprintf("%s-%s-%d-%s-%s", config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)
	}

	fileName = strings.Replace(strings.ToLower(fileName), "_", "-", -1)

	//如果用户配置了测试结果目录需要收集,则下载测试结果,发送到aslan server
	//Note here: p.Task.TestName目前只有默认值test

	testReport := new(types.TestReport)
	if pipelineTask.TestReports == nil {
		pipelineTask.TestReports = make(map[string]interface{})
	}

	if p.Task.JobCtx.TestType == "" {
		p.Task.JobCtx.TestType = setting.FunctionTest
	}

	store, err := s3.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI)
	if err != nil {
		return
	}

	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, pipelineName, pipelineTaskID, "artifact")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "artifact")
	}
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, forcedPathStyle)
	if err == nil {
		if len(p.Task.JobCtx.ArtifactPaths) > 0 {
			prefix := store.GetObjectPath("/")
			if files, err := s3client.ListFiles(store.Bucket, prefix, true); err == nil {
				if len(files) > 0 {
					p.Task.JobCtx.IsHasArtifact = true
				}
			}
		}
	}

	tmpFilename, err := util.GenerateTmpFile()
	if err != nil {
		p.Log.Error(fmt.Sprintf("generate temp file error: %v", err))
	}
	defer func() {
		_ = os.Remove(tmpFilename)
	}()

	store.Subfolder = strings.Replace(store.Subfolder, fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "artifact"), fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "test"), -1)
	objectKey := store.GetObjectPath(fileName)
	err = s3client.Download(store.Bucket, objectKey, tmpFilename)
	if err != nil {
		return
	}

	if p.Task.JobCtx.TestType == setting.FunctionTest && p.Task.JobCtx.TestResultPath != "" {
		b, err := os.ReadFile(tmpFilename)
		if err != nil {
			p.Log.Error(fmt.Sprintf("get test result file error: %v", err))

			msg := "no test result is found"
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}

		err = xml.Unmarshal(b, &testReport.FunctionTestSuite)
		if err != nil {
			msg := fmt.Sprintf("unmarshal result file test suite summary error: %v", err)
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}
		p.Task.ReportReady = true
		testReport.FunctionTestSuite.TestCases = []types.TestCase{}
		//测试报告
		pipelineTask.TestReports[serviceName] = testReport

		if testReport.FunctionTestSuite.Errors > 0 || testReport.FunctionTestSuite.Failures > 0 {
			msg := fmt.Sprintf(
				"%d failure case(s) found",
				testReport.FunctionTestSuite.Errors+testReport.FunctionTestSuite.Failures,
			)
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}

	} else if p.Task.JobCtx.TestType == setting.PerformanceTest {
		csvFile, err := os.Open(tmpFilename)
		if err != nil {
			msg := fmt.Sprintf("get performance test result file error: %v", err)
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}
		defer csvFile.Close()
		csvReader := csv.NewReader(csvFile)
		row, err := csvReader.Read()
		if len(row) != 11 {
			msg := "csv file type match error"
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}

		if err != nil {
			msg := fmt.Sprintf("read performance csv first row error: %v", err)
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}

		rows, err := csvReader.ReadAll()
		if err != nil {
			msg := fmt.Sprintf("read csv readAll error: %v", err)
			p.Log.Error(msg)
			p.Task.Error = msg
			p.Task.TaskStatus = config.StatusFailed
			return
		}
		performanceTestSuites := make([]*types.PerformanceTestSuite, 0)
		for _, row := range rows {
			performanceTestSuite := new(types.PerformanceTestSuite)
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
		p.Task.ReportReady = true
		testReport.PerformanceTestSuites = performanceTestSuites
		//测试报告
		pipelineTask.TestReports[serviceName] = testReport
	}

	// compare failure percent with threshold
	// Integer operation: FAIL/TOTAL > %V  =>  FAIL*100 > TOTAL*V
	//if itReport.TestSuiteSummary.Failures*100 > itReport.TestSuiteSummary.Tests*p.Task.JobCtx.TestThreshold {
	//	p.Task.TaskStatus = task.StatusFailed
	//	return
	//}

}

func (p *TestPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToTestingTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

func (p *TestPlugin) GetTask() interface{} {
	return p.Task
}

func (p *TestPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

func (p *TestPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

func (p *TestPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

func (p *TestPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

func (p *TestPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

func (p *TestPlugin) ResetError() {
	p.Task.Error = ""
}

// Note: Since there are few environment variables and few variables to be replaced,
// this method is temporarily used.
func (p *TestPlugin) renderEnv(data string) string {
	mapper := func(data string) string {
		for _, envar := range p.Task.JobCtx.EnvVars {
			if data != envar.Key {
				continue
			}

			return envar.Value
		}

		return fmt.Sprintf("$%s", data)
	}

	return os.Expand(data, mapper)
}
