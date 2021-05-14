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
	"errors"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type Queue struct {
	pqColl *commonrepo.QueueColl
	log    *xlog.Logger
}

func NewPipelineQueue(log *xlog.Logger) *Queue {
	return &Queue{
		pqColl: commonrepo.NewQueueColl(),
		log:    log,
	}
}

func (q *Queue) List() []*task.Task {
	opt := new(commonrepo.ListQueueOption)
	queues, err := q.pqColl.List(opt)
	if err != nil {
		q.log.Errorf("pqColl.List error: %v", err)
	}
	tasks := make([]*task.Task, 0, len(queues))
	for _, queue := range queues {
		t := ConvertQueueToTask(queue)
		tasks = append(tasks, t)
	}

	return tasks
}

// Push task queue, 如果已经存在相同的 pipeline 并且 multi=false情况 设置新的 pipeline 为 blocked
// 默认task状态为 created
func (q *Queue) Push(pt *task.Task) error {
	if pt == nil {
		return errors.New("nil task")
	}

	if !pt.MultiRun {
		opt := &commonrepo.ListQueueOption{
			PipelineName: pt.PipelineName,
		}
		tasks, err := q.pqColl.List(opt)
		if err == nil && len(tasks) > 0 {
			q.log.Infof("blocked task received: %v %v %v", pt.CreateTime, pt.TaskID, pt.PipelineName)
			pt.Status = config.StatusBlocked
		}
	}

	if err := q.pqColl.Create(ConvertTaskToQueue(pt)); err != nil {
		q.log.Errorf("pqColl.Create error: %v", err)
		return err
	}

	return nil
}

// NextWaitingTask 查询下一个等待的task
func (q *Queue) NextWaitingTask() (*task.Task, error) {
	opt := &commonrepo.ListQueueOption{
		Status: config.StatusWaiting,
	}

	tasks, err := q.pqColl.List(opt)
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

//func (q *Queue) NextBlockedTask() (*task.Task, error) {
//	opt := &commonrepo.ListQueueOption{
//		Status: config.StatusBlocked,
//	}
//
//	tasks, err := q.pqColl.List(opt)
//	if err != nil || len(tasks) == 0 {
//		return nil, errors.New("no blocked task found")
//	}
//	return tasks[0], nil
//}

// BlockedTaskQueue ...
func (q *Queue) BlockedTaskQueue() ([]*task.Task, error) {
	opt := &commonrepo.ListQueueOption{
		Status: config.StatusBlocked,
	}

	queues, err := q.pqColl.List(opt)
	if err != nil || len(queues) == 0 {
		return nil, errors.New("no blocked task found")
	}

	tasks := make([]*task.Task, 0, len(queues))
	for _, queue := range queues {
		t := ConvertQueueToTask(queue)
		tasks = append(tasks, t)
	}

	return tasks, nil
}

// UpdateAgent ...
func (q *Queue) UpdateAgent(taskID int64, pipelineName string, createTime int64, agentID string) error {
	return q.pqColl.UpdateAgent(taskID, pipelineName, createTime, agentID)
}

// Update ...
func (q *Queue) Update(task *task.Task) bool {
	if err := q.pqColl.Update(ConvertTaskToQueue(task)); err != nil {
		return false
	}
	return true
}

// Remove ...
func (q *Queue) Remove(task *task.Task) error {
	if err := q.pqColl.Delete(ConvertTaskToQueue(task)); err != nil {
		q.log.Errorf("pqColl.Delete error: %v", err)
		return err
	}
	return nil
}
