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

	"go.uber.org/zap"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeleteWorkflowV3s(productName, requestID string, log *zap.SugaredLogger) error {
	workflowV3s, _, err := mongodb.NewWorkflowV3Coll().List(&mongodb.ListWorkflowV3Option{ProjectName: productName}, 0, 0)
	if err != nil {
		log.Errorf("WorkflowV3.List error: %s", err)
		return fmt.Errorf("DeleteWorkflowV3s productName %s WorkflowV3.List error: %s", productName, err)
	}
	errList := new(multierror.Error)
	for _, workflowV3 := range workflowV3s {
		if err = DeleteWorkflowV3(workflowV3.ID.Hex(), workflowV3.Name, requestID, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s workflowV3 delete %s error: %s", productName, workflowV3.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func DeleteWorkflowV3(workflowID, workflowName, requestID string, log *zap.SugaredLogger) error {
	taskQueue, err := mongodb.NewQueueColl().List(&mongodb.ListQueueOption{})
	if err != nil {
		log.Errorf("List queued task error: %s", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == workflowName && task.Type == config.WorkflowTypeV3 {
			if err = CancelWorkflowTaskV3("system", task.PipelineName, task.TaskID, config.WorkflowTypeV3, requestID, log); err != nil {
				log.Errorf("task still running, cancel pipelineV3 %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	if err := mongodb.NewWorkflowV3Coll().DeleteByID(workflowID); err != nil {
		log.Errorf("WorkflowV3.delete error: %s", err)
		return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}

	if err := mongodb.NewTaskColl().DeleteByPipelineNameAndType(workflowName, config.WorkflowTypeV3); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %s", err)
	}

	if err := mongodb.NewCounterColl().Delete("WorkflowTaskV3:" + workflowName); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}
