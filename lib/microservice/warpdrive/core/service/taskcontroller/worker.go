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
	"sync"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	plugins "github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Pool struct {
	Tasks []*Task

	concurrency int
	tasksChan   chan *Task
	wg          sync.WaitGroup
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(tasks []*Task, concurrency int) *Pool {
	return &Pool{
		Tasks:       tasks,
		concurrency: concurrency,
		tasksChan:   make(chan *Task),
	}
}

// Run runs all work within the pool and blocks until it's
// finished.
func (p *Pool) Run() {
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}

	p.wg.Add(len(p.Tasks))
	for _, task := range p.Tasks {
		p.tasksChan <- task
	}

	// all workers return
	close(p.tasksChan)

	p.wg.Wait()
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for task := range p.tasksChan {
		task.Run(&p.wg)
	}
}

// Define a type to hold task executor function
type taskOperater func(ctx context.Context, plugin plugins.TaskPlugin, subTask map[string]interface{}, pos int, servicename string, xl *xlog.Logger) (config.Status, error)

// Task encapsulates a taskOperator item that should go in a worker pool
type Task struct {
	//Status holds task status
	Status config.Status
	// Err holds an error that occurred during a task. Its
	// result is only meaningful after Run has been called
	// for the pool that holds it.
	Err error

	ctx              context.Context
	taskExecutorFunc taskOperater
	plugin           plugins.TaskPlugin
	subTask          map[string]interface{}
	pos              int
	servicename      string
	log              *xlog.Logger
}

// NewTask initializes a new task based on a given task operator function
func NewTask(ctx context.Context, taskExecutorFunc taskOperater, plugin plugins.TaskPlugin, subTask map[string]interface{}, pos int, servicename string, log *xlog.Logger) *Task {
	return &Task{
		ctx:              ctx,
		taskExecutorFunc: taskExecutorFunc,
		plugin:           plugin,
		subTask:          subTask,
		pos:              pos,
		servicename:      servicename,
		log:              log,
	}
}

// Run runs a Task and does appropriate accounting via a
// given sync.WorkGroup.
func (t *Task) Run(wg *sync.WaitGroup) {
	t.Status, t.Err = t.taskExecutorFunc(t.ctx, t.plugin, t.subTask, t.pos, t.servicename, t.log)
	defer wg.Done()
}
