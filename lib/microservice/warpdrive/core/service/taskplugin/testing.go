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
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/crypto"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

//InitializeTestTaskPlugin ...
func InitializeTestTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &TestPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

//TestPlugin name should be compatible with task type
type TestPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.Testing
	Log           *xlog.Logger
}

func (p *TestPlugin) SetAckFunc(func()) {
}

const (
	// TestingV2TaskTimeout ...
	TestingV2TaskTimeout = 60 * 60 // 60 minutes
)

// Init ...
func (p *TestPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *TestPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *TestPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *TestPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
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
	p.KubeNamespace = pipelineTask.ConfigPayload.Test.KubeNamespace
	// 重置错误信息
	p.Task.Error = ""
	// 获取测试相关的namespace
	var linkedNamespace string
	var envName string
	if pipelineTask.Type == config.SingleType {
		linkedNamespace = pipelineTask.TaskArgs.Test.Namespace
	} else if pipelineTask.Type == config.WorkflowType {
		product := &types.Product{EnvName: pipelineTask.WorkflowArgs.Namespace, ProductName: pipelineTask.WorkflowArgs.ProductTmplName}
		linkedNamespace = product.ProductName + "-env-" + product.EnvName
		envName = pipelineTask.WorkflowArgs.Namespace
	}

	//if linkedNamespace == "" {
	//	msg := "namespace for testing is empty"
	//	p.Log.Error(msg)
	//	p.Task.TaskStatus = task.StatusFailed
	//	p.Task.Error = msg
	//	return
	//}

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

	testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType,
		pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, pipelineTask.ServiceName)), "_", "-", -1)
	// 测试结果保存名称,两种模式需要区分开
	fileName := testJobName
	if pipelineTask.Type == config.WorkflowType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	} else if pipelineTask.Type == config.TestType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	}
	jobCtx := JobCtxBuilder{
		JobName:     p.JobName,
		PipelineCtx: pipelineCtx,
		ArchiveFile: fileName,
		JobCtx:      p.Task.JobCtx,
		Installs:    p.Task.InstallCtx,
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx.BuildReaperContext(pipelineTask, serviceName))
	if err != nil {
		msg := fmt.Sprintf("cannot reaper.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", p.Type()),
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

	jobImage := fmt.Sprintf("%s-%s", pipelineTask.ConfigPayload.Release.ReaperImage, p.Task.BuildOS)
	if p.Task.ImageFrom == config.ImageFromCustom {
		jobImage = p.Task.BuildOS
	}

	// search namespace should also include desired namespace
	job, err := buildJobWithLinkedNs(
		p.Type(), jobImage, p.JobName, serviceName, p.Task.ResReq, pipelineCtx, pipelineTask, p.Task.Registries,
		p.KubeNamespace,
		linkedNamespace,
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
	if err := createOrUpdateRegistrySecrets(p.KubeNamespace, p.Task.Registries, p.kubeClient); err != nil {
		p.Log.Errorf("create secret error: %v", err)
	}
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create testing job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}
	return
}

// Wait ...
func (p *TestPlugin) Wait(ctx context.Context) {
	status := waitJobEndWithFile(ctx, p.TaskTimeout(), p.KubeNamespace, p.JobName, true, p.kubeClient, p.Log)
	p.SetStatus(status)
}

// Complete ...
func (p *TestPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	pipelineName := pipelineTask.PipelineName
	pipelineTaskID := pipelineTask.TaskID

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// 日志保存失败与否都清理job
	defer func() {
		if p.Task.TaskStatus == config.StatusCancelled || p.Task.TaskStatus == config.StatusTimeout {
			if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
				p.Task.Error = err.Error()
			}
			return
		}
	}()

	err := saveContainerLog(pipelineTask, p.KubeNamespace, p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}
	p.Task.LogFile = p.JobName

	testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType,
		pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, pipelineTask.ServiceName)), "_", "-", -1)

	// 测试结果保存名称,两种模式需要区分开
	fileName := testJobName
	if pipelineTask.Type == config.WorkflowType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	} else if pipelineTask.Type == config.TestType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)
	}

	//如果用户配置了测试结果目录需要收集,则下载测试结果,发送到aslan server
	//Note here: p.Task.TestName目前只有默认值test
	if p.Task.JobCtx.TestResultPath != "" {
		testReport := new(types.TestReport)
		if pipelineTask.TestReports == nil {
			pipelineTask.TestReports = make(map[string]interface{})
		}

		if p.Task.JobCtx.TestType == "" {
			p.Task.JobCtx.TestType = setting.FunctionTest
		}

		var store *s3.S3
		if store, err = s3.NewS3StorageFromEncryptedUri(pipelineTask.StorageUri, crypto.S3key); err == nil {
			if store.Subfolder != "" {
				store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, pipelineName, pipelineTaskID, "artifact")
			} else {
				store.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "artifact")
			}
			if files, err := s3.ListFiles(store, "/", true); err == nil {
				if len(files) > 0 {
					p.Task.JobCtx.IsHasArtifact = true
				}
			}

			tmpFilename, err := util.GenerateTmpFile()
			defer func() {
				_ = os.Remove(tmpFilename)
			}()

			store.Subfolder = strings.Replace(store.Subfolder, fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "artifact"), fmt.Sprintf("%s/%d/%s", pipelineName, pipelineTaskID, "test"), -1)
			if err = s3.Download(context.Background(), store, fileName, tmpFilename); err == nil {
				if p.Task.JobCtx.TestType == setting.FunctionTest {

					b, err := ioutil.ReadFile(tmpFilename)
					if err != nil {
						p.Log.Error(fmt.Sprintf("get test result file error: %v", err))

						msg := "none test result found"
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
						msg := fmt.Sprintf("get performace test result file error: %v", err)
						p.Log.Error(msg)
						p.Task.Error = msg
						p.Task.TaskStatus = config.StatusFailed
						return
					}
					defer csvFile.Close()
					csvReader := csv.NewReader(csvFile)
					row, err := csvReader.Read()
					if len(row) != 11 {
						msg := fmt.Sprintf("csv file type match error")
						p.Log.Error(msg)
						p.Task.Error = msg
						p.Task.TaskStatus = config.StatusFailed
						return
					}

					if err != nil {
						msg := fmt.Sprintf("read performace csv first row error: %v", err)
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
			}
		}
		// compare failure percent with threshold
		// Integer operation: FAIL/TOTAL > %V  =>  FAIL*100 > TOTAL*V
		//if itReport.TestSuiteSummary.Failures*100 > itReport.TestSuiteSummary.Tests*p.Task.JobCtx.TestThreshold {
		//	p.Task.TaskStatus = task.StatusFailed
		//	return
		//}
	}
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

// IsTaskFailed ...
func (p *TestPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *TestPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *TestPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ..
func (p *TestPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *TestPlugin) ResetError() {
	p.Task.Error = ""
}
