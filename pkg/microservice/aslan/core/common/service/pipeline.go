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

package service

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeletePipelines(productName, requestID string, log *zap.SugaredLogger) error {
	pipelines, err := mongodb.NewPipelineColl().List(&mongodb.PipelineListOption{ProductName: productName})
	if err != nil {
		log.Errorf("Pipeline.List error: %v", err)
		return fmt.Errorf("DeletePipelines productName %s Pipeline.List error: %v", productName, err)
	}
	errList := new(multierror.Error)
	for _, pipeline := range pipelines {
		if err = DeletePipeline(pipeline.Name, requestID, true, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s pipeline delete %s error: %v", productName, pipeline.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func DeletePipeline(pipelineName, requestID string, isDeletingProductTmpl bool, log *zap.SugaredLogger) error {
	var pipeline *commonmodels.Pipeline
	var err error
	if !isDeletingProductTmpl {
		pipeline, err = mongodb.NewPipelineColl().Find(&mongodb.PipelineFindOption{Name: pipelineName})
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

	taskQueue, err := mongodb.NewQueueColl().List(&mongodb.ListQueueOption{})
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == pipelineName && task.Type == config.SingleType {
			if err = CancelTaskV2("system", task.PipelineName, task.TaskID, config.SingleType, requestID, log); err != nil {
				log.Errorf("task still running, cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	err = mongodb.NewWorkflowStatColl().Delete(pipelineName, string(config.SingleType))
	if err != nil {
		log.Errorf("WorkflowStat.Delete failed,  error: %v", err)
	}

	if pipeline != nil && pipeline.Hook != nil {
		err = ProcessWebhook(nil, pipeline.Hook.GitHooks, webhook.PipelinePrefix+pipelineName, log)
		if err != nil {
			log.Errorf("Failed to process webhook, err: %s", err)
			return e.ErrCreatePipeline.AddDesc(err.Error())
		}
	}

	if err := mongodb.NewPipelineColl().Delete(pipelineName); err != nil {
		log.Errorf("PipelineV2.Delete error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}

	if err := mongodb.NewTaskColl().DeleteByPipelineNameAndType(pipelineName, config.SingleType); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %v", err)
	}

	if err := mongodb.NewCounterColl().Delete("PipelineTask:" + pipelineName); err != nil {
		log.Errorf("Counter.Delete error: %v", err)
	}

	return nil
}

func GetPipelineInfo(userID int, pipelineName string, log *zap.SugaredLogger) (*commonmodels.Pipeline, error) {
	resp, err := mongodb.NewPipelineColl().Find(&mongodb.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Error(err)
		return resp, e.ErrGetPipeline
	}

	return resp, nil
}

//func GetPipeline(userID int, pipelineName string, log *zap.SugaredLogger) (*commonmodels.Pipeline, error) {
//	resp, err := repo.NewPipelineColl().Find(&repo.PipelineFindOption{Name: pipelineName})
//	if err != nil {
//		log.Error(err)
//		return resp, e.ErrGetPipeline
//	}
//
//	pipe.EnsureSubTasksResp(resp.SubTasks)
//
//	fPipe, err := s.Collections.FavoritePipeline.Find(userID, pipelineName, string(pipe.SingleType))
//	if err == nil && fPipe != nil && fPipe.Name == pipelineName {
//		resp.IsFavorite = true
//	}
//
//	return resp, nil
//}
