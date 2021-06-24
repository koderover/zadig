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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gerrit"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeleteWorkflows(productName, requestID string, log *zap.SugaredLogger) error {
	workflows, err := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{ProductName: productName})
	if err != nil {
		log.Errorf("Workflow.List error: %v", err)
		return fmt.Errorf("DeleteWorkflows productName %s Workflow.List error: %v", productName, err)
	}
	errList := new(multierror.Error)
	for _, workflow := range workflows {
		if err = DeleteWorkflow(workflow.Name, requestID, true, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s workflow delete %s error: %v", productName, workflow.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func DeleteWorkflow(workflowName, requestID string, isDeletingProductTmpl bool, log *zap.SugaredLogger) error {
	taskQueue, err := mongodb.NewQueueColl().List(&mongodb.ListQueueOption{})
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return e.ErrDeletePipeline.AddErr(err)
	}
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == workflowName && task.Type == config.WorkflowType {
			if err = CancelTaskV2("system", task.PipelineName, task.TaskID, config.WorkflowType, requestID, log); err != nil {
				log.Errorf("task still running, cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	// 在删除前，先将workflow查出来，用于删除gerrit webhook
	workflow, err := mongodb.NewWorkflowColl().Find(workflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}

	if !isDeletingProductTmpl {
		prod, err := template.NewProductColl().Find(workflow.ProductTmplName)
		if err != nil {
			log.Errorf("ProductTmpl.Find error: %v", err)
			return e.ErrDeleteWorkflow.AddErr(err)
		}
		if prod.OnboardingStatus != 0 {
			return e.ErrDeleteWorkflow.AddDesc("该工作流所属的项目处于onboarding流程中，不能删除工作流")
		}
	}

	go gerrit.DeleteGerritWebhook(workflow, log)

	//删除所属的所有定时任务
	err = RemoveCronjob(workflowName, log)
	if err != nil {
		return err
	}

	if err := mongodb.NewWorkflowColl().Delete(workflowName); err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}

	if err := mongodb.NewTaskColl().DeleteByPipelineNameAndType(workflowName, config.WorkflowType); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %v", err)
	}

	if deliveryVersions, err := mongodb.NewDeliveryVersionColl().Find(&mongodb.DeliveryVersionArgs{OrgID: 1, WorkflowName: workflowName}); err == nil {
		for _, deliveryVersion := range deliveryVersions {
			if err := mongodb.NewDeliveryVersionColl().Delete(deliveryVersion.ID.Hex()); err != nil {
				log.Errorf("DeleteWorkflow.DeliveryVersion.Delete error: %v", err)
			}

			if err = mongodb.NewDeliveryBuildColl().Delete(deliveryVersion.ID.Hex()); err != nil {
				log.Errorf("DeleteWorkflow.DeliveryBuild.Delete error: %v", err)
			}

			if err = mongodb.NewDeliveryDeployColl().Delete(deliveryVersion.ID.Hex()); err != nil {
				log.Errorf("DeleteWorkflow.DeliveryDeploy.Delete error: %v", err)
			}

			if err = mongodb.NewDeliveryTestColl().Delete(deliveryVersion.ID.Hex()); err != nil {
				log.Errorf("DeleteWorkflow.DeliveryTest.Delete error: %v", err)
			}

			if err = mongodb.NewDeliveryDistributeColl().Delete(deliveryVersion.ID.Hex()); err != nil {
				log.Errorf("DeleteWorkflow.DeliveryDistribute.Delete error: %v", err)
			}
		}
	}

	err = mongodb.NewWorkflowStatColl().Delete(workflowName, string(config.WorkflowType))
	if err != nil {
		log.Errorf("WorkflowStat.Delete failed, error: %v", err)
	}

	if err := mongodb.NewCounterColl().Delete("WorkflowTask:" + workflowName); err != nil {
		log.Errorf("Counter.Delete error: %v", err)
	}
	return nil
}
