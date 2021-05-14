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
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
)

func Clean(task *task.Task) {
	task.ConfigPayload = nil
	task.TaskArgs = nil

	EnsureSubTasksResp(task.SubTasks)
	for _, stage := range task.Stages {
		ensureSubStageResp(stage)
	}
}

func ensureTestTask(subTask map[string]interface{}) (newSub map[string]interface{}, err error) {
	t, err := commonservice.ToTestingTask(subTask)

	if err != nil {
		return
	}

	if len(t.JobCtx.EnvVars) == 0 {
		t.JobCtx.EnvVars = make([]*commonmodels.KeyVal, 0)
	}

	// 隐藏用户设置的敏感信息
	for k := range t.JobCtx.EnvVars {
		if t.JobCtx.EnvVars[k].IsCredential {
			t.JobCtx.EnvVars[k].Value = setting.MaskValue
		}
	}

	for _, repo := range t.JobCtx.Builds {
		repo.OauthToken = setting.MaskValue
	}

	newSub, err = t.ToSubTask()
	if err != nil {
		return
	}

	return newSub, nil
}

func ensureBuildTask(subTask map[string]interface{}) (newSub map[string]interface{}, err error) {
	t, err := commonservice.ToBuildTask(subTask)
	if err != nil {
		return
	}

	// 不需要把安装脚本具体内容传给前端
	t.InstallCtx = nil
	if len(t.JobCtx.EnvVars) == 0 {
		t.JobCtx.EnvVars = make([]*commonmodels.KeyVal, 0)
	}

	if len(t.JobCtx.BuildSteps) == 0 {
		t.JobCtx.BuildSteps = make([]*task.BuildStep, 0)
	}

	// 隐藏用户设置的敏感信息
	for k := range t.JobCtx.EnvVars {
		if t.JobCtx.EnvVars[k].IsCredential {
			t.JobCtx.EnvVars[k].Value = setting.MaskValue
		}
	}

	for _, repo := range t.JobCtx.Builds {
		repo.OauthToken = setting.MaskValue
	}

	newSub, err = t.ToSubTask()
	if err != nil {
		return
	}

	return newSub, nil
}

func ensureJiraTask(subTask map[string]interface{}) (newSub map[string]interface{}, err error) {
	t, err := commonservice.ToJiraTask(subTask)
	if err != nil {
		return
	}

	for _, repo := range t.Builds {
		repo.OauthToken = setting.MaskValue
	}

	newSub, err = t.ToSubTask()
	if err != nil {
		return
	}

	return newSub, nil
}

// EnsureSubTasksResp 确保SubTask中敏感信息和其他不必要信息不返回给前端
func EnsureSubTasksResp(subTasks []map[string]interface{}) {
	for i, subTask := range subTasks {
		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			continue
		}

		switch pre.TaskType {

		case config.TaskBuild:
			if newTask, err := ensureBuildTask(subTask); err == nil {
				subTasks[i] = newTask
			}
		case config.TaskTestingV2:
			if newTask, err := ensureTestTask(subTask); err == nil {
				subTasks[i] = newTask
			}
		case config.TaskJira:
			if newTask, err := ensureJiraTask(subTask); err == nil {
				subTasks[i] = newTask
			}
		}

	}
}

func ensureSubStageResp(stage *commonmodels.Stage) {
	subTasks := stage.SubTasks
	for key, subTask := range subTasks {
		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuild:
			if newTask, err := ensureBuildTask(subTask); err == nil {
				subTasks[key] = newTask
			}
		case config.TaskTestingV2:
			if newTask, err := ensureTestTask(subTask); err == nil {
				subTasks[key] = newTask
			}
		case config.TaskJira:
			if newTask, err := ensureJiraTask(subTask); err == nil {
				subTasks[key] = newTask
			}
		}
	}
}
