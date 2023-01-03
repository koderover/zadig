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
	"fmt"
	"strings"
	"time"

	"github.com/nsqio/go-nsq"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/common"
	plugins "github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/rand"
)

var (
	ctx          context.Context
	cancel       context.CancelFunc
	pipelineTask *task.Task
	pipelineCtx  *task.PipelineCtx
	itReport     *types.ItReport
	xl           *zap.SugaredLogger
)

// TODO: Leave the logic here until we know why it exists.
var durationBeforeNextTask = 10 * time.Second

// Note: `durationTouchMsg` is used to emit `TOUCH` cmd and it should smaller than `durationBeforeNextTask`.
var durationTouchMsg = 5 * time.Second

// Sender: sender to send ack/notification
// TaskPlugins: registered task plugin initiators to initiate specific plugin to execute task
type ExecHandler struct {
	Sender      *nsq.Producer
	TaskPlugins map[config.TaskType]plugins.Initiator
}

type CancelHandler struct{}

// Message handler to handle task execution message
func (h *ExecHandler) HandleMessage(message *nsq.Message) error {
	defer func() {
		// 每次处理完消息, 等待一段时间不处理新消息
		time.Sleep(durationBeforeNextTask)
	}()

	xl = log.SugaredLogger()

	// 获取 PipelineTask 内容
	if err := json.Unmarshal(message.Body, &pipelineTask); err != nil {
		xl.Errorf("unmarshal PipelineTask error: %v", err)
		return nil
	}
	taskName := fmt.Sprintf("%s:%d", pipelineTask.PipelineName, pipelineTask.TaskID)
	xl.Infof("Receiving pipeline task %s message", taskName)

	xl = Logger(pipelineTask)
	ctx, cancel = context.WithCancel(context.Background())

	go func(ctx context.Context, taskName string) {
		for {
			select {
			case <-ctx.Done():
				xl.Infof("Pipeline task %q has been canceled. Exit.", taskName)
				return
			case <-time.After(durationTouchMsg):
				if pipelineTask == nil {
					xl.Infof("Pipeline task %q has completed. Exit.", taskName)
					return
				}

				xl.Infof("After %s, touch message %q.", durationTouchMsg.String(), taskName)
				message.Touch()
			}
		}
	}(ctx, taskName)

	h.runPipelineTask(ctx, cancel, xl)

	// Note: If returning `nil`, we emit `FIN` cmd to nsq indicating that the messsage has been processed succefully.
	return nil
}

func (h *ExecHandler) runPipelineTask(ctx context.Context, cancel context.CancelFunc, xl *zap.SugaredLogger) {
	defer func() {
		h.SendNotification()

		if pipelineTask.Type == config.SingleType || pipelineTask.Type == config.WorkflowType {
			xl.Infof("Pipeline completeGitCheck %s:%d:%s", pipelineTask.PipelineName, pipelineTask.TaskID, pipelineTask.Status)
			if err := completeGitCheck(pipelineTask); err != nil {
				xl.Errorf("completeGitCheck error: %v", err)
			}
		}

		h.SendAck()

		// 重置 task/itrepot 防止新的task Unmarshal到上次内容
		pipelineTask = nil
		pipelineCtx = nil
		itReport = nil
		xl.Info("Pipeline task all done, tear down vars.")
	}()

	// Step 1.1 - 检查配置，如果配置为空，则结束此次Task执行
	if pipelineTask.ConfigPayload == nil {
		errMsg := fmt.Sprintf("Cannot find task config payload %s:%d", pipelineTask.PipelineName, pipelineTask.TaskID)
		xl.Errorf(errMsg)
		pipelineTask.Error = errMsg
		pipelineTask.Status = config.StatusSkipped
		return
	}
	// Step 1.2 - 检查SubTasks和Stages长度，如果配置都为0，则结束此次Task执行
	if len(pipelineTask.SubTasks) == 0 && len(pipelineTask.Stages) == 0 {
		errMsg := fmt.Sprintf("Cannot find sub task to run %s:%d", pipelineTask.PipelineName, pipelineTask.TaskID)
		xl.Infof(errMsg)
		pipelineTask.Error = errMsg
		pipelineTask.Status = config.StatusFailed
		return
	}

	// Deprecated.
	// Note: This logic is reserved for compatibility with plugins other than build. This logic can be removed if we have understood all of the plugins.
	//       For attached clusters, this logic is wrong because it still deals with dind in the local cluster.
	dockerHost, err := plugins.GetBestDockerHost(pipelineTask.ConfigPayload.Docker.HostList, string(pipelineTask.Type), pipelineTask.ConfigPayload.Build.KubeNamespace, xl)
	if err != nil {
		errMsg := fmt.Sprintf("[%s]Cannot find docker host: %v", pipelineTask.PipelineName, err)
		xl.Infof(errMsg)
		pipelineTask.Error = errMsg
		pipelineTask.Status = config.StatusFailed
		return
	}

	// Step 2 - 初始化pipeline task执行全局的Context
	// PipelineCtx -
	// DockerHost: to config job when pod need to set docker daemon socket option
	// Workspace: Pipeline workspace
	// DistDir: pipeline distribute dir
	// DockerMountDir: docker mount dir
	// ConfigMapMountDir: config map mount dir
	pipelineCtx = &task.PipelineCtx{
		DockerHost:        dockerHost,
		Workspace:         fmt.Sprintf("%s/%s", pipelineTask.ConfigPayload.S3Storage.Path, pipelineTask.PipelineName),
		DistDir:           fmt.Sprintf("%s/%s/dist/%d", pipelineTask.ConfigPayload.S3Storage.Path, pipelineTask.PipelineName, pipelineTask.TaskID),
		DockerMountDir:    fmt.Sprintf("/tmp/%s/docker/%d", uuid.NewV4(), time.Now().Unix()),
		ConfigMapMountDir: fmt.Sprintf("/tmp/%s/cm/%d", uuid.NewV4(), time.Now().Unix()),
		MultiRun:          pipelineTask.MultiRun,
	}
	pipelineTask.DockerHost = dockerHost
	// 开始执行Pipeline Task，设置初始化字段和运行状态，包括执行开始时间状态，执行主机
	xl.Infof("start to run pipeline task %s:%d ......", pipelineTask.PipelineName, pipelineTask.TaskID)
	initPipelineTask(pipelineTask, xl)
	h.SendNotification()
	// 发送初始状态ACK给backend，更新pipeline状态
	pipelineTask.Status = config.StatusRunning
	h.SendAck()

	// Step 3 - pipelineTask执行，真的开始了...
	h.execute(ctx, pipelineTask, pipelineCtx, xl)

	// Return 之前会执行defer内容，更新pipeline end time, 发送ACK，发送notification
}

func (h *CancelHandler) HandleMessage(message *nsq.Message) error {
	xl = Logger(pipelineTask)

	// 获取 cancel message
	var msg *CancelMessage
	if err := json.Unmarshal(message.Body, &msg); err != nil {
		xl.Errorf("unmarshal CancelMessage error: %v", err)
		return nil
	}

	xl.Infof("receiving cancel task %s:%d message", msg.PipelineName, msg.TaskID)

	// 如果存在处理的 PipelineTask 并且匹配 PipelineName, 则取消PipelineTask
	if pipelineTask != nil && pipelineTask.PipelineName == msg.PipelineName && pipelineTask.TaskID == msg.TaskID {
		xl.Infof("cancelling message: %+v", msg)
		pipelineTask.TaskRevoker = msg.Revoker

		//取消pipelineTask
		cancel()
		//xl.Debug("no matched pipeline task found on this warpdrive")
	}
	return nil
}

// ----------------------------------------------------------------------------------------------
// helper functions
// ----------------------------------------------------------------------------------------------

// SendAck 发送task实时状态信息
// 无需发送cancel信息
func (h *ExecHandler) SendAck() {
	xl = Logger(pipelineTask)
	pb, err := func() ([]byte, error) {
		pipelineTask.RwLock.Lock()
		defer pipelineTask.RwLock.Unlock()

		pb, err := json.Marshal(&pipelineTask)
		if err != nil {
			return nil, err
		}
		return pb, err
	}()

	if err != nil {
		xl.Errorf("marshal PipelineTask error: %v", err)
		return
	}

	//DEBUG ONLY
	xl.Infof("Sending ACK: %#v", pipelineTask)

	if err := h.Sender.Publish(setting.TopicAck, pb); err != nil {
		xl.Errorf("publish [%s] error: %v", setting.TopicAck, err)
		return
	}
}

// SendItReport ...
func (h *ExecHandler) SendItReport() {
	pb, err := json.Marshal(&itReport)
	if err != nil {
		xl.Errorf("marshal itReport error: %v", err)
		return
	}

	if err := h.Sender.Publish(setting.TopicItReport, pb); err != nil {
		xl.Errorf("publish [%s] error: %v", setting.TopicItReport, err)
		return
	}
}

// SendNotification ...
func (h *ExecHandler) SendNotification() {
	notify := &types.Notify{
		Type:     config.PipelineStatus,
		Receiver: pipelineTask.TaskCreator,
		Content: &types.PipelineStatusCtx{
			TaskID:       pipelineTask.TaskID,
			ProductName:  pipelineTask.ProductName,
			PipelineName: pipelineTask.PipelineName,
			Status:       config.Status(pipelineTask.Status),
			TeamName:     pipelineTask.TeamName,
			Type:         pipelineTask.Type,
			Stages:       pipelineTask.Stages,
		},
		CreateTime: time.Now().Unix(),
		IsRead:     false,
	}

	nb, err := json.Marshal(notify)
	if err != nil {
		xl.Errorf("marshal Notify error: %v", err)
		return
	}
	
	if err := h.Sender.Publish(setting.TopicNotification, nb); err != nil {
		xl.Errorf("publish [%s] error: %v", setting.TopicNotification, err)
		return
	}
}

func (h *ExecHandler) runStage(stagePosition int, stage *common.Stage, concurrency int64) {
	xl.Infof("start to execute pipeline stage: %s at position: %d", stage.TaskType, stagePosition)
	pluginInitiator, ok := h.TaskPlugins[stage.TaskType]
	if !ok {
		xl.Errorf("Error to find plugin initiator to init task plugin of type %s", stage.TaskType)
		return
	}

	xl.Info("start to init worker pool for execute tasks in stage")
	// 初始化stage status为running
	updatePipelineStageStatus(config.StatusRunning, pipelineTask, stagePosition, xl)
	h.SendAck()
	// runParallel: Stage内部是否支持并发
	runParallel := stage.RunParallel
	// Default worker concurrency is 1, run tasks sequentially
	var workerConcurrency = 1
	if runParallel {
		if len(stage.SubTasks) > int(concurrency) {
			workerConcurrency = int(concurrency)
		} else {
			workerConcurrency = len(stage.SubTasks)
		}

	}
	xl.Infof("set worker concurrency to: %d", workerConcurrency)

	// Task is struct for worker
	var tasks []*Task
	//tasks been preprocessed, map[serviceName]=[]Tasks
	pluginsByService := make(map[string]*plugins.HelmDeployTaskPlugin)
	// helm deploy plugins map[fullServiceName]=>HelmDeployPlugin
	helmDeployPlugins := make(map[string]*plugins.HelmDeployTaskPlugin)

	// preprocess subTasks, make batchTask with multiple subTasks
	// eg: multiple deploys of same helm chart
	if stage.TaskType == config.TaskDeploy || stage.TaskType == config.TaskResetImage {
		for fullServiceName, subTask := range stage.SubTasks {
			deployTask, err := plugins.ToDeployTask(subTask)
			if err != nil {
				xl.Errorf("failed to get deplot task, err: %s", err)
				continue
			}
			if deployTask.ServiceType != setting.HelmDeployType {
				continue
			}
			workerConcurrency = 1
			pluginInstance := plugins.InitializeHelmDeployTaskPlugin(config.TaskDeploy)
			pluginInstance.Task = deployTask
			if _, ok := pluginsByService[deployTask.ServiceName]; !ok {
				pluginsByService[deployTask.ServiceName] = pluginInstance
			}
			helmDeployPlugins[fullServiceName] = pluginInstance
		}
	}

	// 每个SubTask会initiate一个plugin instance来执行
	preProcessedServices := sets.NewString()
	for serviceName, subTask := range stage.SubTasks {
		if deployPlugin, ok := helmDeployPlugins[serviceName]; ok {
			svcName := deployPlugin.Task.ServiceName
			pluginsByService[svcName].ContentPlugins = append(pluginsByService[svcName].ContentPlugins, deployPlugin)
			if !preProcessedServices.Has(svcName) {
				xl.Infof("new batch sub task of service name: %s, type: %s", serviceName, stage.TaskType)
				batchTask := NewTask(ctx, h.executeTask, pluginsByService[svcName], subTask, stagePosition, serviceName, xl)
				tasks = append(tasks, batchTask)
			}
			preProcessedServices.Insert(svcName)
			continue
		}

		var pluginInstance plugins.TaskPlugin
		xl.Infof("new sub task of service name: %s, type: %s", serviceName, stage.TaskType)
		pluginInstance = pluginInitiator(stage.TaskType)
		taskObj := NewTask(ctx, h.executeTask, pluginInstance, subTask, stagePosition, serviceName, xl)
		tasks = append(tasks, taskObj)
	}

	// 设置WorkPool来控制最大并发数和并发执行
	workerPool := NewPool(tasks, workerConcurrency)
	// 发起workerConcurrency个并发执行，等待所有Task执行完成并返回
	workerPool.Run()

	// set related task status
	for _, helmDeployPlugin := range helmDeployPlugins {
		if len(helmDeployPlugin.ContentPlugins) == 0 {
			continue
		}
		updatePluginSubTask(helmDeployPlugin, pipelineTask, stagePosition, helmDeployPlugin.Task.ContainerName, xl)
	}

	// Worker is completed
	xl.Info("execution completed of subtasks in stage")
	stageStatus := getStageStatus(workerPool.Tasks, xl)
	xl.Infof("aggregated stage status of stage %d with type %s is: %s", stagePosition, stage.TaskType, stageStatus)
	stage.Status = stageStatus
	// 更新Stage状态
	updatePipelineStageStatus(stage.Status, pipelineTask, stagePosition, xl)
	h.SendAck()
}

// execute: PipelineTask Executor
// 兼容支持1.0和2.0的数据结构
// 支持根据RunParallel参数指定的并发或串行执行
func (h *ExecHandler) execute(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, xl *zap.SugaredLogger) {
	xl.Info("start pipeline task executor...")
	// 如果是pipeline 1.0， 先将subtasks进行transform，转化为stages结构
	if pipelineTask.Type == config.SingleType || pipelineTask.Type == "" || pipelineTask.Type == config.WorkflowTypeV3 {
		err := transformToStages(pipelineTask, xl)
		// 初始化出错时，直接返回pipeline状态错误
		if err != nil {
			xl.Errorf("error when transforming subtasks into stages: %+v", err)
			pipelineTask.Status = config.StatusFailed
			return
		}
	}

	// Only serial is supported between stages
	// If the stage status is StatusFailed/StatusCancelled/StatusTimeout, other than the extension stage will not be executed
	isSkip := false
	for stagePosition, stage := range pipelineTask.Stages {
		if stage.AfterAll {
			continue
		}

		if !isSkip || stage.TaskType == config.TaskExtension {
			h.runStage(stagePosition, stage, pipelineTask.ConfigPayload.BuildConcurrency)
		}

		if stage.Status == config.StatusFailed || stage.Status == config.StatusCancelled || stage.Status == config.StatusTimeout {
			isSkip = true
			continue
		}
	}

	updatePipelineStatus(pipelineTask, xl)
	deployStageStatus := config.StatusInit
	testStageStatus := config.StatusInit
	for stagePosition, stage := range pipelineTask.Stages {
		if stage.TaskType == config.TaskDeploy || stage.TaskType == config.TaskArtifact {
			deployStageStatus = stage.Status
		} else if stage.TaskType == config.TaskTestingV2 {
			testStageStatus = stage.Status
		}
		if stage.AfterAll {
			if stage.TaskType == config.TaskResetImage {
				switch pipelineTask.ResetImagePolicy {
				case setting.ResetImagePolicyTaskCompleted, setting.ResetImagePolicyTaskCompletedOrder:
				case setting.ResetImagePolicyDeployFailed:
					if deployStageStatus == config.StatusInit || (deployStageStatus != config.StatusFailed && deployStageStatus != config.StatusCancelled && deployStageStatus != config.StatusTimeout) {
						continue
					}
				case setting.ResetImagePolicyTestFailed:
					if testStageStatus == config.StatusInit || (testStageStatus != config.StatusFailed && testStageStatus != config.StatusCancelled && testStageStatus != config.StatusTimeout) {
						continue
					}
				}
			}
			h.runStage(stagePosition, stage, pipelineTask.ConfigPayload.BuildConcurrency)
		}
	}

	// 根据stage status汇总pipeline task状态，并且更新pipeline状态，发送ACK
	updatePipelineStatus(pipelineTask, xl)
	h.SendAck()
}

// executeTask
// 执行单个subtask，并将subtask执行状态更新到pipelineTask中
// 返回Task状态+Error，Task Status将在Stage Level进行Aggregation到Stage Status
// SubTask终止状态包括：disabled, passed, skipped, timeout, failed, cancelled.
func (h *ExecHandler) executeTask(taskCtx context.Context, plugin plugins.TaskPlugin, subTask map[string]interface{}, pos int, servicename string, xl *zap.SugaredLogger) (config.Status, error) {
	//设置Plugin执行参数：JOBNAME; 设置plugin logger;设置plugin log文件名称
	//e.g. build task JOBNAME = pipelinename-taskid-buildv2-bsonId
	//e.g. build task FILENAME(singgle模式) = pipelinename-taskid-buildv2-servicename
	//workflow 和 singgle 模式区分开
	//e.g. build task FILENAME(workflow模式) = workflow-pipelinename-taskid-buildv2-servicename
	// JOBNAME makes sure it matches [a-z][.-]
	base := strings.Replace(
		strings.ToLower(
			fmt.Sprintf(
				"%s-%d-%s-",
				pipelineTask.PipelineName,
				pipelineTask.TaskID,
				plugin.Type(),
			),
		),
		"_", "-", -1,
	)

	jobName := rand.GenerateName(base)

	// length of job name should less than 63, the last 6 chars are random string generated for pod
	if len(jobName) > 57 {
		jobName = strings.TrimLeft(jobName[len(jobName)-57:], "-")
	}

	fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type(), servicename)),
		"_", "-", -1)
	if pipelineTask.Type == config.WorkflowType {
		//fileName = fmt.Sprintf("%s-%s", pipeline.WorkflowType, fileName)
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type(), servicename)),
			"_", "-", -1)
	} else if pipelineTask.Type == config.TestType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.TestType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type(), servicename)),
			"_", "-", -1)
	} else if pipelineTask.Type == config.ServiceType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.ServiceType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type(), servicename)),
			"_", "-", -1)
	} else if pipelineTask.Type == config.WorkflowTypeV3 {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.WorkflowTypeV3, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type(), fmt.Sprintf("%s-job", pipelineTask.PipelineName))),
			"_", "-", -1)
	} else if pipelineTask.Type == config.ArtifactType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s", config.ArtifactType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type())),
			"_", "-", -1)
	} else if pipelineTask.Type == config.ScanningType {
		fileName = strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s", config.ScanningType, pipelineTask.PipelineName, pipelineTask.TaskID, plugin.Type())),
			"_", "-", -1)
	}
	plugin.Init(jobName, fileName, xl)

	//设置待执行的subtask
	err := plugin.SetTask(subTask)

	if err != nil {
		xl.Errorf("%v", err)
		return config.StatusFailed, err
	}

	// 不运行disabled的任务
	if !plugin.IsTaskEnabled() {
		return config.StatusDisabled, nil
	}

	// 不需要运行Task status已经是passed的任务，比如重试时
	if plugin.Status() == config.StatusPassed && !getSubTaskTypeAndIsRestart(subTask) {
		if plugin.Type() != config.TaskResetImage {
			return config.StatusPassed, nil
		}
	}

	// 设置 SubTask 初始状态
	switch plugin.Type() {
	case config.TaskBuild, config.TaskTestingV2:
		plugin.SetStatus(config.StatusPrepare)
	default:
		plugin.SetStatus(config.StatusRunning)
	}

	// 设置 SubTask 开始时间
	plugin.SetStartTime()

	// 清除上一次错误信息
	plugin.ResetError()

	updatePluginSubTask(plugin, pipelineTask, pos, servicename, xl)
	h.SendAck()

	plugin.SetAckFunc(func() {
		updatePluginSubTask(plugin, pipelineTask, pos, servicename, xl)
		h.SendAck()
	})

	xl.Info("start to call plugin.Run")
	// 如果是并行跑，用servicename来区分不同的workspace
	runCtx := *pipelineCtx
	if pipelineTask.Type == config.WorkflowType || pipelineTask.Type == config.WorkflowTypeV3 {
		runCtx.Workspace = fmt.Sprintf("%s/%s", pipelineCtx.Workspace, servicename)
	}
	// 运行 SubTask, 如果需要异步，请在方法内实现
	plugin.Run(ctx, pipelineTask, &runCtx, servicename)

	// 如果 SubTask 执行失败, 则不继续执行, 发送 Task 失败执行结果
	// Failed, Timeout, Cancelled
	if plugin.IsTaskFailed() {
		plugin.Complete(ctx, pipelineTask, servicename)
		xl.Infof("task status: %s", plugin.Status())

		plugin.SetEndTime()
		updatePluginSubTask(plugin, pipelineTask, pos, servicename, xl)
		return plugin.Status(), fmt.Errorf("pipeline task failed: task_handler:308")
	}

	xl.Infof("task status: %s", plugin.Status())

	// 等待完成前, 更新 SubTask 执行结果到 PipelineTask
	updatePluginSubTask(plugin, pipelineTask, pos, servicename, xl)
	h.SendAck()

	// 等待 SubTask 结束
	xl.Infof("waiting %s task to complete ...", plugin.Type())
	plugin.Wait(ctx)
	xl.Infof("task status: %s", plugin.Status())

	plugin.Complete(ctx, pipelineTask, servicename)
	xl.Infof("task status: %s", plugin.Status())

	// XXX - TODO需要确认这里的逻辑是？
	if itReport != nil {
		h.SendItReport()
	}
	// 更新 SubTask 执行结果到 PipelineTask
	plugin.SetEndTime()
	updatePluginSubTask(plugin, pipelineTask, pos, servicename, xl)
	h.SendAck()

	xl.Infof("end sub task [%s:%s]", plugin.Type(), plugin.Status())
	return plugin.Status(), nil
}

func Logger(pipelineTask *task.Task) *zap.SugaredLogger {
	l := log.Logger()
	if pipelineTask != nil {
		l.With(zap.String(setting.RequestID, pipelineTask.ReqID))
	}

	return l.Sugar()
}
