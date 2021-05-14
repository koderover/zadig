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

package taskcontroller

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	plugins "github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin/github"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin/github/app"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/tool/crypto"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

// preparePipelineStages
// 1. 转换经过序列化的 SubTasks 到 Stages
// Stage Map: *Stage -> map [service->subtask]
func transformToStages(pipelineTask *task.Task, xl *xlog.Logger) error {
	var pipelineStages []*task.Stage
	// 工作流1.0，单服务工作流，SubTasks一维数组
	// Transform into stages and assign to stages
	// Task的数据结构中如果没有赋值Type，也按照1.0处理
	for _, subTask := range pipelineTask.SubTasks {
		subTaskPreview, err := plugins.ToPreview(subTask)
		if err != nil {
			xl.Errorf("preview error: %v", err)
			pipelineTask.Status = config.StatusFailed
			return err
		}
		// Pipeline 1.0中的一个Subtask对应到Pipeline 2.0中的一个Stage
		// Stage中subtasks仅有一个subtask, key为service_name
		stage := &task.Stage{
			TaskType: subTaskPreview.TaskType,
			// Pipeline 1.0中，每个type subtask只有一个，不存在并行执行
			RunParallel: false,
			SubTasks: map[string]map[string]interface{}{
				pipelineTask.ServiceName: subTask,
			},
		}
		pipelineStages = append(pipelineStages, stage)

	}
	pipelineTask.Stages = pipelineStages
	return nil
}

// 设置开始运行时的pipeline状态；包括执行开始时间、状态为Running，执行主机
func initPipelineTask(pipelineTask *task.Task, xl *xlog.Logger) {
	xl.Infof("start initPipelineTask")
	now := time.Now().Unix()
	pipelineTask.StartTime = now
	pipelineTask.EndTime = now
	pipelineTask.Status = config.StatusRunning

	//设置 Task.AgentHost
	setHostName(pipelineTask)

	if pipelineTask.Type == config.SingleType || pipelineTask.Type == config.WorkflowType {
		// 更新github check from pending to running
		if err := updateGitCheck(pipelineTask); err != nil {
			xl.Errorf("updateGitCheck error: %v", err)
		}
	}
}

// Notes:
// 1.0中pos代表subtasks位置
// 2.0中pos代表stages位置
// 注意：更新1.0 subtask状态时，同时更新了stages
// XXX - TODO, 是否可以不需要更新整个SubTask，仅更新Subtask状态相关字段
func updatePipelineSubTask(t interface{}, pipelineTask *task.Task, pos int, servicename string, xl *xlog.Logger) {
	b, err := json.Marshal(t)
	if err != nil {
		xl.Errorf("marshal Task error: %v", err)
		return
	}
	var subTask map[string]interface{}

	if err := json.Unmarshal(b, &subTask); err != nil {
		xl.Errorf("unmarshal Task error: %v", err)
		return
	}
	xl.Infof("updating pipeline subtask status, service name: %s, subtask position: %d", servicename, pos)

	pipelineTask.RwLock.Lock()
	defer pipelineTask.RwLock.Unlock()

	//没有Type的PipelineTask(老的结构)，按照1.0更新方式处理
	if pipelineTask.Type == config.SingleType || pipelineTask.Type == "" {
		xl.Info("pipeline type is single type: pipeline 1.0")
		// refresh pipeline sub task
		pipelineTask.SubTasks[pos] = subTask
		// 同时更新stages
		// TODO: 完善Stage其他字段
		if len(pipelineTask.Stages) == 0 {
			pipelineTask.Stages = make([]*task.Stage, len(pipelineTask.SubTasks))
		}
		if pipelineTask.Stages[pos] == nil {
			pipelineTask.Stages[pos] = &task.Stage{}
		}
		pipelineTask.Stages[pos].SubTasks = map[string]map[string]interface{}{servicename: subTask}
	} else if pipelineTask.Type == config.WorkflowType {
		xl.Info("pipeline type is workflow type: pipeline 2.0")
		pipelineTask.Stages[pos].SubTasks[servicename] = subTask
	} else if pipelineTask.Type == config.TestType {
		xl.Info("pipeline type is test type: pipeline 3.0")
		pipelineTask.Stages[pos].SubTasks[servicename] = subTask
	} else if pipelineTask.Type == config.ServiceType {
		xl.Info("pipeline type is service type: pipeline 3.0")
		pipelineTask.Stages[pos].SubTasks[servicename] = subTask
	}
}

// updatePipelineStageStatus
// 一个Stage执行结束后，更新PipelineTask的Stage状态
func updatePipelineStageStatus(stageStatus config.Status, pipelineTask *task.Task, pos int, xl *xlog.Logger) {
	xl.Infof("updating pipeline task, stage status: %s, stage position: %d", stageStatus, pos)
	if pipelineTask.Stages[pos] == nil {
		pipelineTask.Stages[pos] = &task.Stage{}
	}
	pipelineTask.Stages[pos].Status = stageStatus
}

func updatePipelineStatus(pipelineTask *task.Task, xl *xlog.Logger) {
	pipelineTask.EndTime = time.Now().Unix()
	//这里不需要处理1.0还是2.0了，因为stage内容已经都更新了，所以根据stage来判断就好
	for _, stage := range pipelineTask.Stages {
		if stage.Status == config.StatusFailed || stage.Status == config.StatusCancelled || stage.Status == config.StatusTimeout {
			pipelineTask.Status = stage.Status
			xl.Infof("Pipeline task completed abnormal: %s:%d:%s %+v", pipelineTask.PipelineName, pipelineTask.TaskID, pipelineTask.Status, pipelineTask)
			return
		}
	}
	pipelineTask.Status = config.StatusPassed
	xl.Infof("Pipeline task completed: %s:%d:%s", pipelineTask.PipelineName, pipelineTask.TaskID, pipelineTask.Status)
	xl.Infof("%+v", pipelineTask)
	return
}

//汇总Stage Status
//制定Status Map，遍历Tasks状态，根据Map赋值。
//最后取值最大的那个状态。
func getStageStatus(tasks []*Task, xl *xlog.Logger) config.Status {
	taskStatusMap := map[config.Status]int{
		config.StatusCancelled: 4,
		config.StatusTimeout:   3,
		config.StatusFailed:    2,
		config.StatusPassed:    1,
		config.StatusSkipped:   0,
	}

	// 初始化stageStatus为创建状态
	stageStatus := config.StatusRunning

	taskStatus := make([]int, len(tasks))

	for i, t := range tasks {
		statusCode, ok := taskStatusMap[t.Status]
		if !ok {
			statusCode = -1
		}
		taskStatus[i] = statusCode
	}
	var stageStatusCode int
	for i, code := range taskStatus {
		if i == 0 || code > stageStatusCode {
			stageStatusCode = code
		}
	}

	for taskstatus, code := range taskStatusMap {
		if stageStatusCode == code {
			stageStatus = taskstatus
			break
		}
	}
	return stageStatus
}

func getCheckStatus(status config.Status) github.CIStatus {
	switch status {
	case config.StatusCreated, config.StatusRunning:
		return github.CIStatusNeutral
	case config.StatusTimeout:
		return github.CIStatusTimeout
	case config.StatusFailed:
		return github.CIStatusFailure
	case config.StatusPassed:
		return github.CIStatusSuccess
	case config.StatusSkipped:
		return github.CIStatusCancelled
	default:
		return github.CIStatusFailure
	}
}

// SetHostName ...
func setHostName(pipelineTask *task.Task) {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "unknown"
	}
	pipelineTask.AgentHost = hostName
}

func initGitClient(pt *task.Task) (*github.Client, error) {
	appCfg := &app.Config{
		AppKey: pt.ConfigPayload.Github.AppKey,
		AppID:  pt.ConfigPayload.Github.AppID,
	}

	appCli, err := app.NewAppClient(appCfg)
	if err != nil {
		return nil, err
	}
	var owner string
	if pt.Type == config.SingleType {
		owner = pt.TaskArgs.HookPayload.Owner
	} else if pt.Type == config.WorkflowType {
		owner = pt.WorkflowArgs.HookPayload.Owner
	}

	installID, err := appCli.FindInstallationID(owner)
	if err != nil {
		return nil, err
	}

	var httpsAddr string
	if pt.ConfigPayload.Proxy.EnableRepoProxy && pt.ConfigPayload.Proxy.Type == "http" {
		httpsAddr = pt.ConfigPayload.Proxy.GetProxyUrl()
	}

	gitCfg := &github.Config{
		AppKey:         pt.ConfigPayload.Github.AppKey,
		AppID:          pt.ConfigPayload.Github.AppID,
		InstallationID: installID,
		ProxyAddr:      httpsAddr,
	}

	return github.NewDynamicClient(gitCfg)
}

func updateGitCheck(pt *task.Task) error {
	// 注意：如果不是PR请求，目前来说只有push请求，则无法更新github status
	// 之后如果有其他类型请求，需要修改
	var hook *task.HookPayload
	if pt.Type == config.SingleType {
		if pt.TaskArgs == nil {
			return nil
		}

		hook = pt.TaskArgs.HookPayload
	} else if pt.Type == config.WorkflowType {
		if pt.WorkflowArgs == nil {
			return nil
		}
		hook = pt.WorkflowArgs.HookPayload
	}

	if hook == nil || !hook.IsPr || hook.CheckRunID == 0 {
		return nil
	}

	gitCli, err := initGitClient(pt)
	if err != nil {
		return err
	}

	opt := &github.GitCheck{
		Owner:  hook.Owner,
		Repo:   hook.Repo,
		Branch: hook.Ref,
		Ref:    hook.Ref,
		IsPr:   hook.IsPr,

		AslanURL:    pt.ConfigPayload.Aslan.URL,
		PipeName:    pt.PipelineName,
		PipeType:    pt.Type,
		ProductName: pt.ProductName,
		TaskID:      pt.TaskID,
	}

	return gitCli.Checks.UpdateGitCheck(hook.CheckRunID, opt)
}

// GitCheck ...
type GitCheck struct {
	Owner  string
	Repo   string
	Branch string // The name of the branch to perform a check against. (Required.)
	Ref    string // The SHA of the commit. (Required.)
	IsPr   bool

	AslanURL    string
	PipeName    string
	ProductName string
	PipeType    config.PipelineType
	TaskID      int64
	TestReports []*types.TestSuite
}

func completeGitCheck(pt *task.Task) error {
	// 注意：如果不是PR请求，目前来说只有push请求，则无法更新github status
	// 之后如果有其他类型请求，需要修改
	var hook *task.HookPayload
	if pt.Type == config.SingleType {
		if pt.TaskArgs == nil {
			return nil
		}

		hook = pt.TaskArgs.HookPayload
	} else if pt.Type == config.WorkflowType {
		if pt.WorkflowArgs == nil {
			return nil
		}
		hook = pt.WorkflowArgs.HookPayload
	}

	if hook == nil || !hook.IsPr || hook.CheckRunID == 0 {
		return nil
	}

	gitCli, err := initGitClient(pt)
	if err != nil {
		return err
	}

	testReports := make([]*types.TestSuite, 0)
	// 从s3下载测试报告
	if pt.Status == config.StatusPassed {
		logger := xlog.NewDummy()
		testReports, err = DownloadTestReports(pt, logger)
		if err != nil {
			logger.Warnf("download testReport from s3 failed,err:%v", err)
		}
	}

	opt := &github.GitCheck{
		Owner:  hook.Owner,
		Repo:   hook.Repo,
		Branch: hook.Ref,
		Ref:    hook.Ref,
		IsPr:   hook.IsPr,

		AslanURL:    pt.ConfigPayload.Aslan.URL,
		PipeName:    pt.PipelineName,
		PipeType:    pt.Type,
		ProductName: pt.ProductName,
		TaskID:      pt.TaskID,
		TestReports: testReports,
	}

	return gitCli.Checks.CompleteGitCheck(hook.CheckRunID, getCheckStatus(pt.Status), opt)
}

func DownloadTestReports(taskInfo *task.Task, logger *xlog.Logger) ([]*types.TestSuite, error) {
	if taskInfo.StorageUri == "" {
		return nil, nil
	}

	testReport := make([]*types.TestSuite, 0)

	switch taskInfo.Type {
	case config.SingleType:
		//testName := taskInfo.TaskArgs.Test.TestModuleName
		fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType,
			taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, taskInfo.ServiceName)), "_", "-", -1)
		testRepo, err := downloadReport(taskInfo, fileName, taskInfo.ServiceName, logger)
		if err != nil {
			return nil, err
		}
		testReport = append(testReport, testRepo)
		return testReport, nil
	case config.WorkflowType:
		if taskInfo.WorkflowArgs == nil {
			return nil, nil
		}
		for _, test := range taskInfo.WorkflowArgs.Tests {
			fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
				config.WorkflowType, taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, test.TestModuleName)), "_", "-", -1)
			testRepo, err := downloadReport(taskInfo, fileName, test.TestModuleName, logger)
			if err != nil {
				return nil, err
			}
			testReport = append(testReport, testRepo)
		}
		return testReport, nil
	case config.TestType:
		if taskInfo.TestArgs == nil {
			return nil, nil
		}
		testName := taskInfo.TestArgs.TestName
		fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.TestType, taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, testName)), "_", "-", -1)
		testRepo, err := downloadReport(taskInfo, fileName, testName, logger)
		if err != nil {
			return nil, err
		}
		testReport = append(testReport, testRepo)
		return testReport, nil
	}

	return nil, nil
}

func downloadReport(taskInfo *task.Task, fileName, testName string, logger *xlog.Logger) (*types.TestSuite, error) {
	var store *s3.S3
	var err error

	if store, err = s3.NewS3StorageFromEncryptedUri(taskInfo.StorageUri, crypto.S3key); err != nil {
		logger.Errorf("failed to create s3 storage %s", taskInfo.StorageUri)
		return nil, err
	}
	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, taskInfo.PipelineName, taskInfo.TaskID, "test")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", taskInfo.PipelineName, taskInfo.TaskID, "test")
	}

	tmpFilename, err := util.GenerateTmpFile()
	defer func() {
		_ = os.Remove(tmpFilename)
	}()

	if err = s3.Download(context.Background(), store, fileName, tmpFilename); err == nil {
		testRepo := new(types.TestSuite)
		b, err := ioutil.ReadFile(tmpFilename)
		if err != nil {
			logger.Error(fmt.Sprintf("get test result file error: %v", err))
			return nil, err
		}

		err = xml.Unmarshal(b, testRepo)
		if err != nil {
			logger.Errorf("unmarshal result file test suite summary error: %v", err)
			return nil, err
		}

		testRepo.Name = testName

		return testRepo, nil
	}

	return nil, err
}

func getSubTaskTypeAndIsRestart(subTask map[string]interface{}) bool {
	if deployInfo, err := plugins.ToDeployTask(subTask); err == nil {
		if deployInfo.IsRestart && deployInfo.ResetImage {
			return true
		}
	} else {
		return false
	}
	return false
}

func initTaskPlugins(execHandler *ExecHandler) {
	pluginConf := map[config.TaskType]plugins.TaskPluginInitiator{
		config.TaskJira:           plugins.InitializeJiraTaskPlugin,
		config.TaskBuild:          plugins.InitializeBuildTaskPlugin,
		config.TaskJenkinsBuild:   plugins.InitializeJenkinsBuildPlugin,
		config.TaskDockerBuild:    plugins.InitializeDockerBuildTaskPlugin,
		config.TaskDeploy:         plugins.InitializeDeployTaskPlugin,
		config.TaskTestingV2:      plugins.InitializeTestTaskPlugin,
		config.TaskSecurity:       plugins.InitializeSecurityPlugin,
		config.TaskReleaseImage:   plugins.InitializeReleaseImagePlugin,
		config.TaskDistributeToS3: plugins.InitializeDistribute2S3TaskPlugin,
		config.TaskResetImage:     plugins.InitializeDeployTaskPlugin,
	}
	for name, pluginInitiator := range pluginConf {
		registerTaskPlugin(execHandler, name, pluginInitiator)
	}
}

// registerTaskPlugin is to register task plugin initiator to handler
func registerTaskPlugin(execHandler *ExecHandler, name config.TaskType, pluginInitiator plugins.TaskPluginInitiator) {
	if execHandler.TaskPlugins == nil {
		execHandler.TaskPlugins = make(map[config.TaskType]plugins.TaskPluginInitiator)
	}
	execHandler.TaskPlugins[name] = pluginInitiator
}
