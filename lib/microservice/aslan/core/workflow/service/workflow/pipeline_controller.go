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

package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	nsqservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/lib/setting"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func SubScribeNSQ(log *xlog.Logger) error {
	// init ack consumer
	ackConfig := nsqservice.Config()
	ackConfig.MaxInFlight = 50
	ackHandler := NewTaskAckHandler(config.PoetryAPIServer(), config.PoetryAPIRootKey(), ackConfig.MaxInFlight, log)
	err := nsqservice.SubScribe(setting.TopicAck, "ack", 1, ackConfig, ackHandler)
	if err != nil {
		log.Errorf("ack subscription failed, the error is: %v", err)
		return err
	}

	// init itReport consumer
	itReportHandler := &ItReportHandler{
		itReportColl: commonrepo.NewItReportColl(),
		log:          log,
	}
	err = nsqservice.SubScribeSimple(setting.TopicItReport, "it.report", itReportHandler)
	if err != nil {
		log.Errorf("itReport subscription failed, the error is: %v", err)
		return err
	}

	// init notification consumer
	notificationCfg := nsqservice.Config()
	notificationCfg.MaxInFlight = 50
	notificationHandler := &TaskNotificationHandler{
		log: log,
	}
	err = nsqservice.SubScribe(setting.TopicNotification, "notification", 1, notificationCfg, notificationHandler)
	if err != nil {
		log.Errorf("notification subscription failed, the error is: %v", err)
		return err
	}

	return nil
}

func RunningTasks() []*task.Task {
	tasks := make([]*task.Task, 0)
	queueTasks, err := commonrepo.NewQueueColl().List(&commonrepo.ListQueueOption{})
	if err != nil {
		log.Errorf("list queue task failed, err:%v", err)
		return tasks
	}
	for _, t := range queueTasks {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status == config.StatusRunning {
			if t, err := commonrepo.NewTaskColl().Find(t.TaskID, t.PipelineName, t.Type); err == nil {
				Clean(t)
				tasks = append(tasks, t)
			}
		}
	}
	return tasks
}

func PendingTasks() []*task.Task {
	tasks := make([]*task.Task, 0)
	queueTasks, err := commonrepo.NewQueueColl().List(&commonrepo.ListQueueOption{})
	if err != nil {
		log.Errorf("list queue task failed, err:%v", err)
		return tasks
	}
	for _, queueTask := range queueTasks {
		if queueTask.Status == config.StatusWaiting || queueTask.Status == config.StatusBlocked || queueTask.Status == config.StatusQueued {
			t := ConvertQueueToTask(queueTask)
			Clean(t)

			if t.Stages == nil {
				t.Stages = make([]*commonmodels.Stage, 0)
			}

			tasks = append(tasks, t)
		}
	}
	return tasks
}

type CancelMessage struct {
	Revoker      string `json:"revoker"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
	ReqID        string `json:"req_id"`
}

//func CancelTask(userName, pipelineName string, taskID int64, typeString config.PipelineType, reqID string) error {
//	log := xlog.NewDummy()
//	queueTasks, err := commonrepo.NewQueueColl().List(&commonrepo.ListQueueOption{})
//	if err != nil {
//		log.Errorf("list queue task failed, err:%v", err)
//		return err
//	}
//
//	for _, t := range queueTasks {
//		if t.PipelineName == pipelineName &&
//			t.TaskID == taskID &&
//			t.Type == typeString &&
//			(t.Status == config.StatusWaiting || t.Status == config.StatusBlocked || t.Status == config.StatusCreated) {
//			commonservice.Remove(t, log)
//
//			err := commonrepo.NewTaskColl().UpdateStatus(taskID, pipelineName, userName, config.StatusCancelled)
//			if err != nil {
//				log.Errorf("PipelineTaskV2.UpdateStatus [%s:%d] error: %v", pipelineName, taskID, err)
//				return err
//			}
//
//			// 获取更新后的任务数据并发送通知
//			t, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, typeString)
//			if err != nil {
//				log.Errorf("[%s] task: %s:%d not found", userName, pipelineName, taskID)
//				return err
//			}
//
//			notifyCli := scmnotify.NewService()
//			if typeString == config.WorkflowType {
//				notifyCli.UpdateWebhookComment(t, log)
//				notifyCli.UpdateDiffNote(t, log)
//			} else if typeString == config.TestType {
//				notifyCli.UpdateWebhookCommentForTest(t, log)
//			} else if typeString == config.SingleType {
//				notifyCli.UpdatePipelineWebhookComment(t, log)
//			}
//
//			return nil
//		}
//	}
//
//	b, err := json.Marshal(CancelMessage{Revoker: userName, PipelineName: pipelineName, TaskID: taskID, ReqID: reqID})
//	if err != nil {
//		log.Errorf("marshal cancel message error: %v", err)
//		return err
//	}
//
//	t, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, typeString)
//	if err != nil {
//		log.Errorf("[%s] task: %s:%d not found", userName, pipelineName, taskID)
//		return err
//	}
//
//	if t.Status == config.StatusPassed {
//		log.Errorf("[%s] task: %s:%d is passed, cannot cancel", userName, pipelineName, taskID)
//		return fmt.Errorf("task: %s:%d is passed, cannot cancel", pipelineName, taskID)
//	}
//
//	t.Status = config.StatusCancelled
//	t.TaskRevoker = userName
//
//	log.Infof("[%s] CancelRunningTask %s:%d on %s", userName, pipelineName, taskID, t.AgentHost)
//
//	if err := commonrepo.NewTaskColl().Update(t); err != nil {
//		log.Errorf("[%s] update task: %s:%d error: %v", userName, pipelineName, taskID, err)
//		return err
//	}
//
//	commonservice.Remove(ConvertTaskToQueue(t), log)
//
//	if err := nsqservice.Publish(config.TopicCancel, b); err != nil {
//		log.Errorf("[%s] cancel %s %s:%d error", userName, t.AgentHost, pipelineName, taskID)
//		return err
//	}
//
//	notifyCli := scmnotify.NewService()
//	if typeString == config.WorkflowType {
//		notifyCli.UpdateWebhookComment(t, log)
//		notifyCli.UpdateDiffNote(t, log)
//	} else if typeString == config.TestType {
//		notifyCli.UpdateWebhookCommentForTest(t, log)
//	} else if typeString == config.SingleType {
//		notifyCli.UpdatePipelineWebhookComment(t, log)
//	}
//
//	return nil
//}

// CreateTask 接受create task请求, 保存task到数据库, 发送task到queue
func CreateTask(t *task.Task) error {
	if err := commonrepo.NewTaskColl().Create(t); err != nil {
		log.Errorf("create PipelineTaskV2 error: %v", err)
		return err
	}
	t.Status = config.StatusWaiting
	return Push(t)
}

func UpdateTask(t *task.Task) error {
	if err := commonrepo.NewTaskColl().Update(t); err != nil {
		log.Errorf("create PipelineTaskV2 error: %v", err)
		return err
	}

	t.Status = config.StatusWaiting
	return Push(t)
}

func Push(pt *task.Task) error {
	if pt == nil {
		return errors.New("nil task")
	}

	if !pt.MultiRun {
		opt := &commonrepo.ListQueueOption{
			PipelineName: pt.PipelineName,
		}
		tasks, err := commonrepo.NewQueueColl().List(opt)
		if err == nil && len(tasks) > 0 {
			log.Infof("blocked task recevied: %v %v %v", pt.CreateTime, pt.TaskID, pt.PipelineName)
			pt.Status = config.StatusBlocked
		}
	}

	if err := commonrepo.NewQueueColl().Create(ConvertTaskToQueue(pt)); err != nil {
		log.Errorf("pqColl.Create error: %v", err)
		return err
	}

	return nil
}

func InitPipelineController() {
	InitQueue()
	go PipelineTaskSender()
}

func InitQueue() error {
	log := xlog.NewDummy()

	// 从数据库查找未完成的任务
	// status = created, running
	tasks, err := commonrepo.NewTaskColl().InCompletedTasks()
	if err != nil {
		log.Errorf("find [InCompletedTasks] error: %v", err)
		return err
	}

	for _, task := range tasks {
		// 如果 Queue 重新初始化, 取消所有 running tasks
		if err := commonservice.CancelTask(setting.DefaultTaskRevoker, task.PipelineName, task.TaskID, task.Type, task.ReqID, log); err != nil {
			log.Errorf("[CancelRunningTask] error: %v", err)
			continue
		}
	}

	return nil
}

// PipelineTaskSender 监控warpdrive空闲情况, 如果有空闲, 则发现下一个waiting task给warpdrive
// 并将task状态设置为queued
func PipelineTaskSender() {
	for {
		time.Sleep(time.Second * 3)

		//c.checkAgents()
		if hasAgentAvaiable() {
			t, err := NextWaitingTask()
			if err != nil {
				// no waiting task found
				blockTasks, err := BlockedTaskQueue()
				if err != nil {
					//no blocked task found
					continue
				}
				for _, blockTask := range blockTasks {
					if hasAgentAvaiable() {
						runningTasksMap := make(map[string]bool)
						for _, running := range RunningAndQueuedTasks() {
							runningTasksMap[running.PipelineName] = true
						}

						if _, ok := runningTasksMap[blockTask.PipelineName]; ok {
							//判断相同的工作流是否有相同的服务需要更新
							if !ParallelRunningAndQueuedTasks(blockTask) {
								continue
							}
						}
						// update agent and queue
						if err := updateAgentAndQueue(blockTask); err != nil {
							continue
						}
					} else {
						break
					}
				}
				continue
			}
			// update agent and queue
			if err := updateAgentAndQueue(t); err != nil {
				continue
			}
		}
	}
}

func hasAgentAvaiable() bool {
	return len(RunningAndQueuedTasks()) < agentCount()
}

func RunningAndQueuedTasks() []*task.Task {
	tasks := make([]*task.Task, 0)
	for _, t := range ListTasks() {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status == config.StatusRunning || t.Status == config.StatusQueued {
			Clean(t)
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func ListTasks() []*task.Task {
	opt := new(commonrepo.ListQueueOption)
	queues, err := commonrepo.NewQueueColl().List(opt)
	if err != nil {
		log.Errorf("pqColl.List error: %v", err)
	}

	tasks := make([]*task.Task, 0, len(queues))
	for _, queue := range queues {
		task := ConvertQueueToTask(queue)
		tasks = append(tasks, task)
	}

	return tasks
}

func agentCount() int {
	var (
		parallelLimit int = -1
		wdReplicas    int = 0
	)

	kubeClient := krkubeclient.Client()
	deployment, _, err := getter.GetDeployment(config.Namespace(), config.ENVWarpdriveService(), kubeClient)
	if err != nil {
		log.Errorf("kubeCli.GetDeployment error: %v", err)
		return 0
	}
	wdReplicas = int(deployment.Status.ReadyReplicas)

	if parallelLimit == -1 || parallelLimit >= wdReplicas {
		return wdReplicas
	} else if 0 < parallelLimit && parallelLimit < wdReplicas {
		return parallelLimit
	}
	return 0
}

// NextWaitingTask 查询下一个等待的task
func NextWaitingTask() (*task.Task, error) {
	opt := &commonrepo.ListQueueOption{
		Status: config.StatusWaiting,
	}

	tasks, err := commonrepo.NewQueueColl().List(opt)
	if err != nil {
		return nil, err
	}

	for _, t := range tasks {
		if t.AgentID == "" {
			return ConvertQueueToTask(t), nil
		}
	}

	return nil, errors.New("no waiting task found")
}

func BlockedTaskQueue() ([]*task.Task, error) {
	opt := &commonrepo.ListQueueOption{
		Status: config.StatusBlocked,
	}

	queues, err := commonrepo.NewQueueColl().List(opt)
	if err != nil || len(queues) == 0 {
		return nil, errors.New("no blocked task found")
	}

	tasks := make([]*task.Task, 0, len(queues))
	for _, queue := range queues {
		task := ConvertQueueToTask(queue)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func ParallelRunningAndQueuedTasks(currentTask *task.Task) bool {
	//只有产品工作流单一工作流任务支持并发，其他的类型暂不支持
	if currentTask.Type == config.SingleType || currentTask.Type == config.TestType || currentTask.Type == config.ServiceType {
		return false
	}
	//如果是产品工作流判断是否设置了并行
	if currentTask.Type == config.WorkflowType && !currentTask.WorkflowArgs.IsParallel {
		return false
	}
	//优先处理webhook
	if currentTask.TaskCreator == setting.WebhookTaskCreator {
		return HandlerWebhookTask(currentTask)
	}
	//判断如果当前的task没有部署，直接放到队列里执行
	currentTaskDeployServices := sets.NewString()
	for _, subStage := range currentTask.Stages {
		taskType := subStage.TaskType
		switch taskType {
		case config.TaskDeploy:
			subDeployTaskMap := subStage.SubTasks
			for serviceName := range subDeployTaskMap {
				currentTaskDeployServices.Insert(serviceName)
			}
		}
	}
	if len(currentTaskDeployServices) == 0 {
		return true
	}
	queueTasks := make([]*task.Task, 0)
	for _, t := range ListTasks() {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status != config.StatusRunning && t.Status != config.StatusQueued {
			continue
		}
		if t.PipelineName == currentTask.PipelineName {
			queueTasks = append(queueTasks, t)
			continue
		}
	}
	for _, queueTask := range queueTasks {
		for _, subStage := range queueTask.Stages {
			if subStage.TaskType != config.TaskDeploy {
				continue
			}
			subDeployTaskMap := subStage.SubTasks
			for serviceName := range subDeployTaskMap {
				if currentTaskDeployServices.Has(serviceName) {
					return false
				}
			}
		}
	}
	return true
}

// HandlerWebhookTask 处理webhook逻辑
func HandlerWebhookTask(currentTask *task.Task) bool {
	runningNamespaces := sets.String{}
	for _, t := range ListTasks() {
		if t.Status != config.StatusRunning && t.Status != config.StatusQueued {
			continue
		}
		if t.PipelineName != currentTask.PipelineName {
			continue
		}

		if !runningNamespaces.Has(t.WorkflowArgs.Namespace) {
			runningNamespaces.Insert(t.WorkflowArgs.Namespace)
		}
	}
	if !runningNamespaces.Has(currentTask.WorkflowArgs.Namespace) && currentTask.WorkflowArgs.IsParallel {
		return true
	}

	return false
}

func updateAgentAndQueue(t *task.Task) error {
	if err := UpdateTaskAgent(t.TaskID, t.PipelineName, t.CreateTime, config.PodName()); err != nil {
		log.Infof("task %v/%v may processed by aonther instance: %v", t.TaskID, t.PipelineName, err)
		return err
	}

	b, err := json.Marshal(t)
	if err != nil {
		log.Errorf("marshal PipelineTaskV2 error: %v", err)
		return err
	}

	// 发送当前任务到nsq
	log.Infof("sending task to warpdrive %s:%d", t.PipelineName, t.TaskID)
	if err = nsqservice.Publish(config.TopicProcess, b); err != nil {
		log.Errorf("Publish %s:%d to nsq error: %v", t.PipelineName, t.TaskID, err)
		return err
	}
	// 更新当前任务状态为 TaskQueued
	t.Status = config.StatusQueued
	// 更新队列状态为TaskQueued
	if success := UpdateQueue(t); !success {
		log.Errorf("%s:%d update t status error", t.PipelineName, t.TaskID)
		return fmt.Errorf("%s:%d update t status error", t.PipelineName, t.TaskID)
	}
	return nil
}

func UpdateQueue(task *task.Task) bool {
	if err := commonrepo.NewQueueColl().Update(ConvertTaskToQueue(task)); err != nil {
		return false
	}
	return true
}

func UpdateTaskAgent(taskID int64, pipelineName string, createTime int64, agentID string) error {
	return commonrepo.NewQueueColl().UpdateAgent(taskID, pipelineName, createTime, agentID)
}
