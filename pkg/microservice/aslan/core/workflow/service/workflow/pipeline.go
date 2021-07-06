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
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListPipelines(log *zap.SugaredLogger) ([]*commonmodels.Pipeline, error) {
	resp, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("list PipelineV2 error: %v", err)
		return resp, e.ErrListPipeline
	}
	for i := range resp {
		EnsureSubTasksResp(resp[i].SubTasks)
	}
	return resp, nil
}

func GetPipeline(userID int, pipelineName string, log *zap.SugaredLogger) (*commonmodels.Pipeline, error) {
	resp, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Error(err)
		return resp, e.ErrGetPipeline
	}

	EnsureSubTasksResp(resp.SubTasks)

	fPipe, err := commonrepo.NewFavoriteColl().Find(userID, pipelineName, string(config.SingleType))
	if err == nil && fPipe != nil && fPipe.Name == pipelineName {
		resp.IsFavorite = true
	}

	return resp, nil
}

func UpsertPipeline(args *commonmodels.Pipeline, log *zap.SugaredLogger) error {
	if !checkPipelineSubModules(args) {
		errStr := "pipeline没有子模块，请先设置子模块"
		return e.ErrCreatePipeline.AddDesc(errStr)
	}

	log.Debugf("Start to create or update pipeline %s", args.Name)
	if err := ensurePipeline(args, log); err != nil {
		return e.ErrCreatePipeline.AddDesc(err.Error())
	}

	var currentHooks, updatedHooks []commonmodels.GitHook
	currentPipeline, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: args.Name})
	if err == nil && currentPipeline != nil && currentPipeline.Hook != nil {
		currentHooks = currentPipeline.Hook.GitHooks
	}
	if args.Hook != nil {
		updatedHooks = args.Hook.GitHooks
	}

	err = processWebhook(updatedHooks, currentHooks, webhook.PipelinePrefix+args.Name, log)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
		return e.ErrCreatePipeline.AddDesc(err.Error())
	}

	if err := commonrepo.NewPipelineColl().Upsert(args); err != nil {
		log.Error(err)
		return e.ErrCreatePipeline.AddDesc(err.Error())
	}

	return nil
}

func checkPipelineSubModules(args *commonmodels.Pipeline) bool {
	if args.SubTasks != nil && len(args.SubTasks) > 0 {
		return true
	}
	if args.Hook != nil && args.Hook.Enabled {
		return true
	}
	if args.Schedules.Enabled {
		return true
	}
	if args.Slack != nil && args.Slack.Enabled {
		return true
	}
	if args.NotifyCtl != nil && args.NotifyCtl.Enabled {
		return true
	}

	return false
}

func CopyPipeline(oldPipelineName, newPipelineName, username string, log *zap.SugaredLogger) error {
	oldPipeline, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: oldPipelineName})
	if err != nil {
		log.Error(err)
		return e.ErrGetPipeline
	}
	_, err = commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: newPipelineName})
	if err == nil {
		log.Error("new pipeline already exists")
		return e.ErrExistsPipeline
	}
	oldPipeline.UpdateBy = username
	oldPipeline.Name = newPipelineName
	return UpsertPipeline(oldPipeline, log)
}

func RenamePipeline(oldName, newName string, log *zap.SugaredLogger) error {
	if len(oldName) == 0 || len(newName) == 0 {
		return e.ErrRenamePipeline.AddDesc("pipeline name cannot be empty")
	}

	// 检查新名字格式
	if !defaultNameRegex.MatchString(newName) {
		log.Errorf("pipeline name must match %s", defaultNameRegexString)
		return fmt.Errorf("%s %s", e.InvalidFormatErrMsg, defaultNameRegexString)
	}

	taskQueue, err := commonrepo.NewQueueColl().List(&commonrepo.ListQueueOption{})
	if err != nil {
		return e.ErrRenamePipeline.AddErr(err)
	}

	// 当task还在运行时，不能rename pipeline
	for _, task := range taskQueue {
		if task.PipelineName == oldName {
			return e.ErrRenamePipeline.AddDesc("task still running,can not rename pipeline")
		}
	}

	// 新名字的pipeline已经存在，不能rename pipeline
	opt := &commonrepo.PipelineFindOption{Name: newName}
	if _, err := commonrepo.NewPipelineColl().Find(opt); err == nil {
		return e.ErrRenamePipeline.AddDesc("newname has already existed, can not rename pipeline")
	}

	if err := commonrepo.NewPipelineColl().Rename(oldName, newName); err != nil {
		log.Errorf("Pipeline.Rename %s -> %s error: %v", oldName, newName, err)
		return e.ErrRenamePipeline.AddErr(err)
	}

	if err := commonrepo.NewTaskColl().Rename(oldName, newName, config.SingleType); err != nil {
		log.Errorf("PipelineTask.Rename %s -> %s error: %v", oldName, newName, err)
		return e.ErrRenamePipeline.AddErr(err)
	}

	if err := commonrepo.NewCounterColl().Rename(
		fmt.Sprintf(setting.PipelineTaskFmt, oldName),
		fmt.Sprintf(setting.PipelineTaskFmt, newName)); err != nil && err.Error() != "not found" {
		log.Errorf("Counter.Rename %s -> %s error: %v", oldName, newName, err)
		return e.ErrRenamePipeline.AddErr(err)
	}

	return nil
}

func DeletePipeline(pipelineName, requestID string, isDeletingProductTmpl bool, log *zap.SugaredLogger) error {
	if !isDeletingProductTmpl {
		pipeline, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
		if err != nil {
			log.Errorf("Pipeline.Find error: %v", err)
			return e.ErrDeletePipeline.AddErr(err)
		}
		prod, err := template.NewProductColl().Find(pipeline.ProductName)
		if err != nil {
			log.Errorf("ProductTmpl.Find error: %v", err)
			return e.ErrDeletePipeline.AddErr(err)
		}
		if prod.OnboardingStatus != 0 {
			return e.ErrDeletePipeline.AddDesc("该工作流所属的项目处于onboarding流程中，不能删除工作流")
		}
	}

	opt := new(commonrepo.ListQueueOption)
	taskQueue, err := commonrepo.NewQueueColl().List(opt)
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == pipelineName && task.Type == config.SingleType {
			if err = commonservice.CancelTaskV2("system", task.PipelineName, task.TaskID, config.SingleType, requestID, log); err != nil {
				log.Errorf("task still running, cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	err = commonrepo.NewWorkflowStatColl().Delete(pipelineName, string(config.SingleType))
	if err != nil {
		log.Errorf("WorkflowStat.Delete failed,  error: %v", err)
	}

	if err := commonrepo.NewPipelineColl().Delete(pipelineName); err != nil {
		log.Errorf("PipelineV2.Delete error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}

	if err := commonrepo.NewTaskColl().DeleteByPipelineNameAndType(pipelineName, config.SingleType); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %v", err)
	}

	if err := commonrepo.NewCounterColl().Delete("PipelineTask:" + pipelineName); err != nil {
		log.Errorf("Counter.Delete error: %v", err)
	}

	return nil
}
