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

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"

	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func DeletePipelines(productName string, log *xlog.Logger) error {
	pipelines, err := repo.NewPipelineColl().List(&repo.PipelineListOption{ProductName: productName})
	if err != nil {
		log.Errorf("Pipeline.List error: %v", err)
		return fmt.Errorf("DeletePipelines productName %s Pipeline.List error: %v", productName, err)
	}
	errList := new(multierror.Error)
	for _, pipeline := range pipelines {
		if err = DeletePipeline(pipeline.Name, true, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s pipeline delete %s error: %v", productName, pipeline.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func DeletePipeline(pipelineName string, isDeletingProductTmpl bool, log *xlog.Logger) error {
	if !isDeletingProductTmpl {
		pipeline, err := repo.NewPipelineColl().Find(&repo.PipelineFindOption{Name: pipelineName})
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

	taskQueue, err := repo.NewQueueColl().List(&repo.ListQueueOption{})
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == pipelineName && task.Type == config.SingleType {
			if err = CancelTaskV2("system", task.PipelineName, task.TaskID, config.SingleType, log); err != nil {
				log.Errorf("task still running, cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	err = repo.NewWorkflowStatColl().Delete(pipelineName, string(config.SingleType))
	if err != nil {
		log.Errorf("WorkflowStat.Delete failed,  error: %v", err)
	}

	if err := repo.NewPipelineColl().Delete(pipelineName); err != nil {
		log.Errorf("PipelineV2.Delete error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}

	if err := repo.NewTaskColl().DeleteByPipelineNameAndType(pipelineName, config.SingleType); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %v", err)
	}

	if err := repo.NewCounterColl().Delete("PipelineTask:" + pipelineName); err != nil {
		log.Errorf("Counter.Delete error: %v", err)
	}

	return nil
}

func GetPipelineInfo(userID int, pipelineName string, log *xlog.Logger) (*commonmodels.Pipeline, error) {
	resp, err := repo.NewPipelineColl().Find(&repo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Error(err)
		return resp, e.ErrGetPipeline
	}

	return resp, nil
}

//func GetPipeline(userID int, pipelineName string, log *xlog.Logger) (*commonmodels.Pipeline, error) {
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
