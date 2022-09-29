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
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/workflowcontroller"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func DeleteWorkflowV4sByProjectName(projectName string, log *zap.SugaredLogger) error {
	workflowV4s, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{ProjectName: projectName}, 0, 0)
	if err != nil {
		log.Errorf("WorkflowV4.List error: %s", err)
		return fmt.Errorf("DeleteWorkflowV4s productName %s WorkflowV4.List error: %s", projectName, err)
	}
	errList := new(multierror.Error)
	for _, workflowV4 := range workflowV4s {
		if err = DeleteWorkflowV4(workflowV4.Name, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s workflowV4 delete %s error: %s", projectName, workflowV4.Name, err))
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
	err = ProcessWebhook(nil, workflow.HookCtls, webhook.WorkflowV4Prefix+workflow.Name, logger)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
	}

	err = DisableCronjobForWorkflowV4(workflow)
	if err != nil {
		log.Errorf("Failed to stop cronjob for workflowv4: %s, error: %s", workflow.Name, err)
	}

	go gerrit.DeleteGerritWebhookForWorkflowV4(workflow, logger)

	err = mongodb.NewCronjobColl().Delete(&mongodb.CronjobDeleteOption{
		ParentName: name,
		ParentType: config.WorkflowV4Cronjob,
	})
	if err != nil {
		log.Errorf("Failed to delete cronjob for workflowV4 %s, error: %s", workflow.Name, err)
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

func EncryptKeyVals(encryptedKey string, kvs []*commonmodels.KeyVal, logger *zap.SugaredLogger) error {
	if encryptedKey == "" {
		return nil
	}
	aesKey, err := GetAesKeyFromEncryptedKey(encryptedKey, logger)
	if err != nil {
		log.Errorf("EncypteKeyVals GetAesKeyFromEncryptedKey err:%v", err)
		return err
	}
	for _, kv := range kvs {
		if kv.IsCredential {
			kv.Value, err = crypto.AesEncryptByKey(kv.Value, aesKey.PlainText)
			if err != nil {
				log.Errorf("aes encrypt by key err:%v", err)
				return err
			}
		}
	}
	return nil
}

func EncryptParams(encryptedKey string, params []*commonmodels.Param, logger *zap.SugaredLogger) error {
	if encryptedKey == "" {
		return nil
	}
	aesKey, err := GetAesKeyFromEncryptedKey(encryptedKey, logger)
	if err != nil {
		log.Errorf("EncyptParams GetAesKeyFromEncryptedKey err:%v", err)
		return err
	}
	for _, param := range params {
		if param.IsCredential {
			param.Value, err = crypto.AesEncryptByKey(param.Value, aesKey.PlainText)
			if err != nil {
				log.Errorf("aes encrypt by key err:%v", err)
				return err
			}
		}
	}
	return nil
}

func DisableCronjobForWorkflowV4(workflow *commonmodels.WorkflowV4) error {
	disableIDList := make([]string, 0)
	payload := &CronjobPayload{
		Name:    workflow.Name,
		JobType: config.WorkflowCronjob,
		Action:  setting.TypeEnableCronjob,
	}

	jobList, err := mongodb.NewCronjobColl().List(&mongodb.ListCronjobParam{
		ParentName: workflow.Name,
		ParentType: config.WorkflowV4Cronjob,
	})
	if err != nil {
		return err
	}

	for _, job := range jobList {
		disableIDList = append(disableIDList, job.ID.Hex())
	}
	payload.DeleteList = disableIDList

	pl, _ := json.Marshal(payload)
	return nsq.Publish(setting.TopicCronjob, pl)
}
