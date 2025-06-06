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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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

	gitHooks, err := mongodb.NewWorkflowV4GitHookColl().List(internalhandler.NewBackgroupContext(), workflow.Name)
	if err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}

	err = ProcessWebhook(nil, gitHooks, webhook.WorkflowV4Prefix+workflow.Name, logger)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
	}

	err = DisableCronjobForWorkflowV4(workflow)
	if err != nil {
		log.Errorf("Failed to stop cronjob for workflowv4: %s, error: %s", workflow.Name, err)
	}

	go gerrit.DeleteGerritWebhookForWorkflowV4(gitHooks, logger)

	err = mongodb.NewCronjobColl().Delete(&mongodb.CronjobDeleteOption{
		ParentName: name,
		ParentType: setting.WorkflowV4Cronjob,
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

func EncryptKeyVals(encryptedKey string, kvs commonmodels.RuntimeKeyValList, logger *zap.SugaredLogger) error {
	if encryptedKey == "" {
		return nil
	}
	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, logger)
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
	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, logger)
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
		JobType: setting.WorkflowCronjob,
		Action:  setting.TypeEnableCronjob,
	}

	jobList, err := mongodb.NewCronjobColl().List(&mongodb.ListCronjobParam{
		ParentName: workflow.Name,
		ParentType: setting.WorkflowV4Cronjob,
	})
	if err != nil {
		return err
	}

	for _, job := range jobList {
		disableIDList = append(disableIDList, job.ID.Hex())
	}
	payload.DeleteList = disableIDList

	pl, _ := json.Marshal(payload)
	return mongodb.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
}

func FillServiceModules2Jobs(args *commonmodels.WorkflowV4) (*commonmodels.WorkflowV4, bool, error) {
	change := 0
	for _, stage := range args.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped || (job.ServiceModules != nil && len(job.ServiceModules) > 0) {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				services := make([]*commonmodels.WorkflowServiceModule, 0)
				build := new(commonmodels.ZadigBuildJobSpec)
				if err := commonmodels.IToi(job.Spec, build); err != nil {
					return nil, false, err
				}
				for _, serviceAndBuild := range build.ServiceAndBuilds {
					sm := &commonmodels.WorkflowServiceModule{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   serviceAndBuild.ServiceName,
							ServiceModule: serviceAndBuild.ServiceModule,
						},
					}
					for _, repo := range serviceAndBuild.Repos {
						sm.CodeInfo = append(sm.CodeInfo, repo)
					}
					services = append(services, sm)
				}
				if len(services) > 0 {
					change++
					job.ServiceModules = services
				}
			}

			if job.JobType == config.JobZadigDeploy {
				services := make([]*commonmodels.WorkflowServiceModule, 0)
				deploy := new(commonmodels.ZadigDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, deploy); err != nil {
					return nil, false, err
				}
				for _, serviceInfo := range deploy.Services {
					for _, moduleInfo := range serviceInfo.Modules {
						sm := &commonmodels.WorkflowServiceModule{
							ServiceWithModule: commonmodels.ServiceWithModule{
								ServiceName:   serviceInfo.ServiceName,
								ServiceModule: moduleInfo.ServiceModule,
							},
						}
						services = append(services, sm)
					}
				}
				if len(services) > 0 {
					change++
					job.ServiceModules = services
				}
			}

			if job.JobType == config.JobZadigTesting {
				services := make([]*commonmodels.WorkflowServiceModule, 0)
				testing := new(commonmodels.ZadigTestingJobSpec)
				if err := commonmodels.IToi(job.Spec, testing); err != nil {
					return nil, false, err
				}
				for _, serviceAndTesting := range testing.ServiceAndTests {
					sm := &commonmodels.WorkflowServiceModule{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   serviceAndTesting.ServiceName,
							ServiceModule: serviceAndTesting.ServiceModule,
						},
					}
					services = append(services, sm)
				}
				if len(services) > 0 {
					change++
					job.ServiceModules = services
				}
			}
		}
	}
	if change > 0 {
		return args, true, nil
	}
	return args, false, nil
}
