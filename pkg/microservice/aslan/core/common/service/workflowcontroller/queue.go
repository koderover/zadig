/*
Copyright 2022 The KodeRover Authors.

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

package workflowcontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func RunningTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	queueTasks, err := commonrepo.NewWorkflowQueueColl().List(&commonrepo.ListWorfklowQueueOption{})
	if err != nil {
		log.Errorf("list queue worklfow task failed, err:%v", err)
		return tasks
	}
	for _, t := range queueTasks {
		if t.Status == config.StatusRunning || t.Status == config.StatusWaitingApprove {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func PendingTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	queueTasks, err := commonrepo.NewWorkflowQueueColl().List(&commonrepo.ListWorfklowQueueOption{})
	if err != nil {
		log.Errorf("list queue workflow task failed, err:%v", err)
		return tasks
	}
	for _, t := range queueTasks {
		if t.Status == config.StatusWaiting || t.Status == config.StatusBlocked || t.Status == config.StatusQueued {
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

// CreateTask 接受create task请求, 保存task到数据库, 发送task到queue
func CreateTask(t *commonmodels.WorkflowTask) error {
	t.Status = config.StatusWaiting
	if _, err := commonrepo.NewworkflowTaskv4Coll().Create(t); err != nil {
		log.Errorf("create workflow task v4 error: %v", err)
		return err
	}
	return Push(t)
}

func UpdateTask(t *commonmodels.WorkflowTask) error {
	t.Status = config.StatusWaiting
	if err := commonrepo.NewworkflowTaskv4Coll().Update(t.ID.Hex(), t); err != nil {
		log.Errorf("update workflow task v4 %s error: %v", t.WorkflowName, err)
		return err
	}
	return Push(t)
}

func Push(t *commonmodels.WorkflowTask) error {
	if t == nil {
		return errors.New("nil task")
	}

	if err := commonrepo.NewWorkflowQueueColl().Create(ConvertTaskToQueue(t)); err != nil {
		log.Errorf("workflowTaskV4.Create error: %v", err)
		return err
	}
	return nil
}

func InitWorkflowController() {
	InitQueue()
	go WorfklowTaskSender()
}

func InitQueue() error {
	log := log.SugaredLogger()

	// 从数据库查找未完成的任务
	// status = created, running
	tasks, err := commonrepo.NewworkflowTaskv4Coll().InCompletedTasks()
	if err != nil {
		log.Errorf("find [InCompletedTasks] error: %v", err)
		return err
	}

	for _, task := range tasks {
		// 如果 Queue 重新初始化, 取消所有 running tasks
		if err := CancelWorkflowTask(setting.DefaultTaskRevoker, task.WorkflowName, task.TaskID, log); err != nil {
			log.Errorf("[CancelRunningTask] error: %v", err)
			continue
		}
	}

	// clear all cancel pipeline task msgs when aslan restart
	err = commonrepo.NewMsgQueueCommonColl().DeleteByQueueType(setting.TopicCancel)
	if err != nil {
		log.Warnf("remove cancel msgs error: %v", err)
	}
	return nil
}

const (
	workflowTaskSenderLockKey = "workflow-task-sender"
	// Idle replicas poll the queue every 3 seconds.
	workflowTaskSenderIdleInterval = 3 * time.Second
	// Shorter interval while a backlog remains, so another replica can take the next batch.
	workflowTaskSenderBusyInterval = 200 * time.Millisecond
	// Max workflows released per round, so one replica does not start the whole backlog.
	workflowTaskSenderBatchSize = 10
	// redsync does not renew the lock. Expiry must cover this batch of database updates.
	workflowTaskSenderLockExpiry = 30 * time.Second
)

// WorfklowTaskSender holds the distributed lock and releases waiting tasks within the global quota and per-workflow concurrency limit.
// Each round releases at most workflowTaskSenderBatchSize tasks. They run on the replica that acquired the lock.
func WorfklowTaskSender() {
	for {
		time.Sleep(runWorkflowTaskSenderOnce())
	}
}

func runWorkflowTaskSenderOnce() time.Duration {
	mutex := cache.NewRedisTryLock(workflowTaskSenderLockKey, workflowTaskSenderLockExpiry)
	if err := mutex.TryLock(); err != nil {
		if cache.IsLockTaken(err) {
			return workflowTaskSenderBusyInterval
		}
		return workflowTaskSenderIdleInterval
	}
	defer mutex.Unlock()

	moreWaiting, err := dispatchWaitingWorkflowTasks()
	if err == nil && moreWaiting {
		return workflowTaskSenderBusyInterval
	}
	return workflowTaskSenderIdleInterval
}

func dispatchWaitingWorkflowTasks() (bool, error) {
	sysSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("get system stettings error: %v", err)
		return false, err
	}

	waiting, globalUsed, occupied := queueScheduleState(ListTasks())
	slots := dispatchSlots(int(sysSetting.WorkflowConcurrency), globalUsed)
	if slots == 0 || len(waiting) == 0 {
		return false, nil
	}

	limits := make(map[string]int)
	var resolveErr error
	indexes, moreWaiting := pickDispatchBatch(len(waiting), func(i int) (string, int, bool) {
		if resolveErr != nil {
			return "", 0, true
		}
		task := waiting[i]
		if limit, ok := limits[task.WorkflowName]; ok {
			return task.WorkflowName, limit, false
		}
		limit, remove, err := taskConcurrencyLimit(task)
		if err != nil {
			resolveErr = err
			return "", 0, true
		}
		if remove {
			return "", 0, true
		}
		limits[task.WorkflowName] = limit
		return task.WorkflowName, limit, false
	}, occupied, slots)

	started := 0
	for _, idx := range indexes {
		if err := updateQueueAndRunTask(waiting[idx], int(sysSetting.BuildConcurrency)); err != nil {
			continue
		}
		started++
	}
	if resolveErr != nil {
		return false, resolveErr
	}
	if started == 0 {
		return false, nil
	}
	return moreWaiting, nil
}

// queueScheduleState splits the queue into tasks waiting to be released, global slots in use, and slots in use per workflow.
// queued counts toward both the global quota and the workflow quota, so a task that is not yet running is not released again.
func queueScheduleState(tasks []*commonmodels.WorkflowQueue) (waiting []*commonmodels.WorkflowQueue, globalUsed int, occupied map[string]int) {
	occupied = make(map[string]int)
	for _, task := range tasks {
		switch task.Status {
		case config.StatusRunning, config.StatusQueued:
			globalUsed++
			occupied[task.WorkflowName]++
		case config.StatusWaitingApprove:
			occupied[task.WorkflowName]++
		case config.StatusWaiting:
			waiting = append(waiting, task)
		}
	}
	return waiting, globalUsed, occupied
}

func dispatchSlots(globalLimit, globalUsed int) int {
	slots := globalLimit - globalUsed
	if slots > workflowTaskSenderBatchSize {
		slots = workflowTaskSenderBatchSize
	}
	if slots < 0 {
		return 0
	}
	return slots
}

// pickDispatchBatch selects task indexes to release this round, in queue order.
// limitOf returns -1 when the workflow has no concurrency limit. skip means leave the task for later.
// Once the round is full, stop reading further tasks. moreWaiting means unseen tasks remain, and the next round should use the short interval.
func pickDispatchBatch(count int, limitOf func(i int) (name string, limit int, skip bool), occupied map[string]int, slots int) ([]int, bool) {
	if slots <= 0 || count <= 0 {
		return nil, false
	}
	indexes := make([]int, 0, slots)
	for i := 0; i < count; i++ {
		if len(indexes) >= slots {
			return indexes, true
		}
		name, limit, skip := limitOf(i)
		if skip {
			continue
		}
		if limit != -1 && occupied[name] >= limit {
			continue
		}
		indexes = append(indexes, i)
		if limit != -1 {
			occupied[name]++
		}
	}
	return indexes, false
}

// taskConcurrencyLimit returns the concurrency limit of the workflow that owns the task.
// remove means the config is missing and the task has been deleted from the queue.
// A non-nil error means the lookup failed. The task stays queued, and the caller should stop this round and return to the idle interval.
func taskConcurrencyLimit(task *commonmodels.WorkflowQueue) (int, bool, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(task.WorkflowName)
	if err != nil {
		log.Errorf("WorkflowV4 Queue: find workflow %s error: %v", task.WorkflowName, err)
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, err
		}
		switch task.Type {
		case config.WorkflowTaskTypeScanning:
			segs := strings.Split(task.WorkflowName, "-")
			if len(segs) != 3 {
				log.Errorf("invalid scanning workflow name: %s", task.WorkflowName)
				Remove(task)
				return 0, true, nil
			}
			scanningInfo, scanErr := commonrepo.NewScanningColl().GetByID(segs[2])
			if scanErr != nil {
				log.Errorf("failed to find scanning of id: %s, error: %s", segs[2], scanErr)
				if !isMissingWorkflowConfig(scanErr) {
					return 0, false, scanErr
				}
				Remove(task)
				return 0, true, nil
			}
			concurrencyNum := -1
			if scanningInfo.AdvancedSetting != nil {
				concurrencyNum = scanningInfo.AdvancedSetting.ConcurrencyLimit
			}
			if concurrencyNum == 0 {
				concurrencyNum = -1
			}
			return concurrencyNum, false, nil
		case config.WorkflowTaskTypeTesting:
			testingInfo, testErr := commonrepo.NewTestingColl().Find(task.WorkflowDisplayName, task.ProjectName)
			if testErr != nil {
				log.Errorf("failed to find test of name: %s in project: %s, error: %s", task.WorkflowDisplayName, task.ProjectName, testErr)
				if !isMissingWorkflowConfig(testErr) {
					return 0, false, testErr
				}
				Remove(task)
				return 0, true, nil
			}
			concurrencyNum := -1
			if testingInfo.PreTest != nil {
				concurrencyNum = testingInfo.PreTest.ConcurrencyLimit
			}
			if concurrencyNum == 0 {
				concurrencyNum = -1
			}
			return concurrencyNum, false, nil
		case config.WorkflowTaskTypeDelivery:
			return -1, false, nil
		default:
			log.Errorf("unsupported task type: %s, removing from queue", task.Type)
			Remove(task)
			return 0, true, nil
		}
	}
	return workflow.ConcurrencyLimit, false, nil
}

func isMissingWorkflowConfig(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, primitive.ErrInvalidHex)
}

func RunningAndQueuedTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	for _, t := range ListTasks() {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status == config.StatusRunning || t.Status == config.StatusQueued {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func ListTasks() []*commonmodels.WorkflowQueue {
	opt := new(commonrepo.ListWorfklowQueueOption)
	queues, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		log.Errorf("pqColl.List error: %v", err)
	}
	return queues
}

// WaitingTasks 查询所有等待的task
func WaitingTasks() ([]*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		Status: config.StatusWaiting,
	}

	tasks, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		return nil, err
	}

	if len(tasks) > 0 {
		return tasks, nil
	}
	return nil, errors.New("no waiting task found")
}

func RunningWorkflowTasks(name string) ([]*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		WorkflowName: name,
		Status:       config.StatusRunning,
	}

	tasks, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return tasks, nil
}

func WaitForApproveWorkflowTasks(name string) ([]*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		WorkflowName: name,
		Status:       config.StatusWaitingApprove,
	}

	tasks, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return tasks, nil
}

func BlockedTaskQueue() ([]*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		Status: config.StatusBlocked,
	}

	queues, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil || len(queues) == 0 {
		return nil, errors.New("no blocked task found")
	}

	return queues, nil
}

func updateQueueAndRunTask(t *commonmodels.WorkflowQueue, jobConcurrency int) error {
	logger := log.SugaredLogger()
	// 更新队列状态为TaskQueued
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(t.WorkflowName, t.TaskID)
	if err != nil {
		logger.Errorf("%s:%d get workflow task error: %v", t.WorkflowName, t.TaskID, err)
		return fmt.Errorf("%s:%d get workflow task error: %v", t.WorkflowName, t.TaskID, err)
	}
	workflowTask.Status = config.StatusQueued
	if success := UpdateQueue(workflowTask); !success {
		logger.Errorf("%s:%d update t status error", t.WorkflowName, t.TaskID)
		return fmt.Errorf("%s:%d update t status error", t.WorkflowName, t.TaskID)
	}

	ctx := context.Background()
	go NewWorkflowController(workflowTask, logger).Run(ctx, jobConcurrency)
	return nil
}

func UpdateQueue(task *commonmodels.WorkflowTask) bool {
	if err := commonrepo.NewWorkflowQueueColl().Update(ConvertTaskToQueue(task)); err != nil {
		return false
	}
	return true
}

func ConvertTaskToQueue(task *commonmodels.WorkflowTask) *commonmodels.WorkflowQueue {
	return &commonmodels.WorkflowQueue{
		TaskID:              task.TaskID,
		WorkflowName:        task.WorkflowName,
		WorkflowDisplayName: task.WorkflowDisplayName,
		ProjectName:         task.ProjectName,
		Status:              task.Status,
		Stages:              cleanStages(task.Stages),
		TaskCreator:         task.TaskCreator,
		TaskRevoker:         task.TaskRevoker,
		CreateTime:          task.CreateTime,
		Type:                task.Type,
	}
}

func cleanStages(stages []*commonmodels.StageTask) []*commonmodels.StageTask {
	resp := []*commonmodels.StageTask{}
	data, _ := json.Marshal(stages)
	json.Unmarshal(data, &resp)
	for _, stage := range resp {
		for _, job := range stage.Jobs {
			job.Spec = nil
		}
	}
	return resp
}

func Remove(taskQueue *commonmodels.WorkflowQueue) error {
	return commonrepo.NewWorkflowQueueColl().Delete(taskQueue)
}
