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

package webhook

import (
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func AutoCancelTask(autoCancelOpt *AutoCancelOpt, log *xlog.Logger) error {
	if autoCancelOpt == nil || autoCancelOpt.MergeRequestID == "" || autoCancelOpt.CommitID == "" {
		return nil
	}

	tasks, err := commonrepo.NewTaskColl().FindTodoTasks()
	if err != nil {
		log.Errorf("find [InCompletedTasks] error: %v", err)
		return err
	}

	for _, task := range tasks {
		if task.TaskCreator != setting.WebhookTaskCreator ||
			task.Type != autoCancelOpt.TaskType ||
			task.TriggerBy == nil {
			continue
		}

		// 判断当前任务和上一个任务是不是由同一个代码库的同一个pr触发的
		// 不是同一个仓库的同一个pr，跳过
		if autoCancelOpt.MainRepo.CodehostID != task.TriggerBy.CodehostID ||
			autoCancelOpt.MainRepo.RepoOwner != task.TriggerBy.RepoOwner ||
			autoCancelOpt.MainRepo.RepoName != task.TriggerBy.RepoName ||
			autoCancelOpt.MergeRequestID != task.TriggerBy.MergeRequestID {
			continue
		}

		// 同一个pr下的任务，如果commitID相同，说明是本次commit触发了多个同类型的任务，不能互相取消，需要跳过
		if task.TriggerBy.CommitID == autoCancelOpt.CommitID {
			continue
		}
		if task.Type == config.WorkflowType {
			if err := AutoCancelWorkflowTask(autoCancelOpt, task, log); err != nil {
				log.Errorf("auto cancel workflow task failed, old task id:%d, workflow name:%s, mergeRequestID:%s, err:%v", task.TaskID, task.PipelineName, autoCancelOpt.MergeRequestID, err)
				continue
			}
		} else if task.Type == config.TestType {
			if err := AutoCancelTestTask(autoCancelOpt, task, log); err != nil {
				log.Errorf("auto cancel test task failed, old task id:%d, test name:%s, mergeRequestID:%s, err:%v", task.TaskID, task.PipelineName, autoCancelOpt.MergeRequestID, err)
				continue
			}
		}
	}

	return nil
}

func AutoCancelWorkflowTask(autoCancelOpt *AutoCancelOpt, task *task.Task, log *xlog.Logger) error {
	if autoCancelOpt == nil || task == nil || task.WorkflowArgs == nil {
		return nil
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(task.WorkflowArgs.WorkflowName)
	if err != nil {
		log.Errorf("find workflow failed, workflow name:%s, error: %v", task.WorkflowArgs.WorkflowName, err)
		return err
	}

	if workflow.HookCtl == nil || len(workflow.HookCtl.Items) == 0 {
		return nil
	}

	for _, item := range workflow.HookCtl.Items {
		if item.AutoCancel {
			if item.MainRepo.RepoOwner != autoCancelOpt.MainRepo.RepoOwner ||
				item.MainRepo.RepoName != autoCancelOpt.MainRepo.RepoName ||
				item.MainRepo.Branch != autoCancelOpt.MainRepo.Branch ||
				item.MainRepo.Source != autoCancelOpt.MainRepo.Source {
				continue
			}

			if err := commonservice.CancelTask(task.TaskCreator, task.PipelineName, task.TaskID, task.Type, task.ReqID, log); err != nil {
				log.Errorf("CancelRunningTask failed,task.TaskCreator:%s, task.PipelineName:%s, task.TaskID:%d, task.Type:%s, error: %v", task.TaskCreator, task.PipelineName, task.TaskID, task.Type, err)
				continue
			}
		}
	}
	return nil
}

func AutoCancelTestTask(autoCancelOpt *AutoCancelOpt, task *task.Task, log *xlog.Logger) error {
	if autoCancelOpt == nil || task == nil || task.TestArgs == nil {
		return nil
	}

	test, err := commonrepo.NewTestingColl().Find(task.TestArgs.TestName, task.TestArgs.ProductName)
	if err != nil {
		log.Errorf("find test failed, test name:%s, error: %v", task.TestArgs.TestName, err)
		return err
	}

	if test.HookCtl == nil || len(test.HookCtl.Items) == 0 {
		return nil
	}

	for _, item := range test.HookCtl.Items {
		if item.AutoCancel {
			if item.MainRepo.RepoOwner != autoCancelOpt.MainRepo.RepoOwner ||
				item.MainRepo.RepoName != autoCancelOpt.MainRepo.RepoName ||
				item.MainRepo.Branch != autoCancelOpt.MainRepo.Branch ||
				item.MainRepo.Source != autoCancelOpt.MainRepo.Source {
				continue
			}
			if err := commonservice.CancelTask(task.TaskCreator, task.PipelineName, task.TaskID, task.Type, task.ReqID, log); err != nil {
				log.Errorf("CancelRunningTask failed,task.TaskCreator:%s, task.PipelineName:%s, task.TaskID:%d, task.Type:%s, error: %v", task.TaskCreator, task.PipelineName, task.TaskID, task.Type, err)
				continue
			}
		}
	}

	return nil
}
