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

package service

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/workflowcontroller"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"go.uber.org/zap"
)

func DeleteWorkflowV4s(productName string, log *zap.SugaredLogger) error {
	workflowV4s, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{ProjectName: productName}, 0, 0)
	if err != nil {
		log.Errorf("WorkflowV4.List error: %s", err)
		return fmt.Errorf("DeleteWorkflowV4s productName %s WorkflowV4.List error: %s", productName, err)
	}
	errList := new(multierror.Error)
	for _, workflowV4 := range workflowV4s {
		if err = DeleteWorkflowV4(workflowV4.Name, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s workflowV4 delete %s error: %s", productName, workflowV4.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func DeleteWorkflowV4(name string, logger *zap.SugaredLogger) error {
	taskQueue, err := mongodb.NewWorkflowQueueColl().List(&mongodb.ListWorfklowQueueOption{})
	if err != nil {
		log.Errorf("List queued task error: %s", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// when task still running,cancel task first.
	for _, task := range taskQueue {
		if task.WorkflowName == name {
			if err = workflowcontroller.CancelWorkflowTask("system", task.WorkflowName, task.TaskID, logger); err != nil {
				log.Errorf("task still running, cancel pipelineV4 %s task %d", task.WorkflowName, task.TaskID)
			}
		}
	}
	workflow, err := mongodb.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := mongodb.NewWorkflowV4Coll().DeleteByID(workflow.ID.Hex()); err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := mongodb.NewworkflowTaskv4Coll().DeleteByWorkflowName(name); err != nil {
		logger.Errorf("Failed to delete WorkflowV4 task: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := mongodb.NewCounterColl().Delete("WorkflowTaskV4:" + name); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}
