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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func DeleteWorkflows(productName, requestID string, log *zap.SugaredLogger) error {
	workflows, err := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{Projects: []string{productName}})
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
	// query workflow before deleting, used to delete gerrit webhook
	workflow, err := mongodb.NewWorkflowColl().Find(workflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return err
	}
	workflowCMMap, err := collaboration.GetWorkflowCMMap([]string{workflow.ProductTmplName}, log)
	if err != nil {
		return err
	}
	if cmSets, ok := workflowCMMap[collaboration.BuildWorkflowCMMapKey(workflow.ProductTmplName, workflowName)]; ok {
		return fmt.Errorf("this is a base workflow, collaborations:%v is related", cmSets.List())
	}
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

	err = ProcessWebhook(nil, workflow.HookCtl.Items, webhook.WorkflowPrefix+workflow.Name, log)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
	}

	err = DisableCronjobForWorkflow(workflow)
	if err != nil {
		log.Errorf("Failed to stop cronjob for workflow: %s, error: %s", workflow.Name, err)
	}

	go gerrit.DeleteGerritWebhook(workflow, log)

	//删除所属的所有定时任务
	err = mongodb.NewCronjobColl().Delete(&mongodb.CronjobDeleteOption{
		ParentName: workflowName,
		ParentType: setting.WorkflowCronjob,
	})
	if err != nil {
		log.Errorf("Failed to delete cronjob for workflow %s, error: %s", workflow.Name, err)
		//return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}

	if err := mongodb.NewWorkflowColl().Delete(workflowName); err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}

	if err := mongodb.NewTaskColl().DeleteByPipelineNameAndType(workflowName, config.WorkflowType); err != nil {
		log.Errorf("PipelineTaskV2.DeleteByPipelineName error: %v", err)
	}

	if deliveryVersions, _, err := mongodb.NewDeliveryVersionColl().Find(&mongodb.DeliveryVersionArgs{WorkflowName: workflowName}); err == nil {
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

func DisableCronjobForWorkflow(workflow *models.Workflow) error {
	disableIDList := make([]string, 0)
	payload := &CronjobPayload{
		Name:    workflow.Name,
		JobType: setting.WorkflowCronjob,
		Action:  setting.TypeEnableCronjob,
	}
	if workflow.ScheduleEnabled {
		jobList, err := mongodb.NewCronjobColl().List(&mongodb.ListCronjobParam{
			ParentName: workflow.Name,
			ParentType: setting.WorkflowCronjob,
		})
		if err != nil {
			return err
		}

		for _, job := range jobList {
			disableIDList = append(disableIDList, job.ID.Hex())
		}
		payload.DeleteList = disableIDList
	}

	pl, _ := json.Marshal(payload)
	return mongodb.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
}

func ProcessWebhook(updatedHooks, currentHooks interface{}, name string, logger *zap.SugaredLogger) error {
	currentSet := toHookSet(currentHooks)
	updatedSet := toHookSet(updatedHooks)
	hooksToRemove := currentSet.Difference(updatedSet)
	hooksToAdd := updatedSet.Difference(currentSet)

	if hooksToRemove.Len() > 0 {
		logger.Debugf("Going to remove webhooks %+v", hooksToRemove)
	}
	if hooksToAdd.Len() > 0 {
		logger.Debugf("Going to add webhooks %+v", hooksToAdd)
	}

	var errs *multierror.Error
	var wg sync.WaitGroup

	for _, h := range hooksToRemove {
		wg.Add(1)
		go func(wh hookItem) {
			defer wg.Done()
			ch, err := systemconfig.New().GetRawCodeHost(wh.codeHostID)
			if err != nil {
				logger.Errorf("Failed to get codeHost by id %d, err: %s", wh.codeHostID, err)
				errs = multierror.Append(errs, err)
				return
			}

			switch ch.Type {
			case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
				err = webhook.NewClient().RemoveWebHook(&webhook.TaskOption{
					ID:          ch.ID,
					Name:        wh.name,
					Owner:       wh.owner,
					Namespace:   wh.namespace,
					Repo:        wh.repo,
					Address:     ch.Address,
					Token:       ch.AccessToken,
					AK:          ch.AccessKey,
					SK:          ch.SecretKey,
					Region:      ch.Region,
					EnableProxy: ch.EnableProxy,
					Ref:         name,
					From:        ch.Type,
					IsManual:    wh.IsManual,
				})
				if err != nil {
					logger.Errorf("Failed to remove %s webhook %+v, err: %s", ch.Type, wh, err)
					errs = multierror.Append(errs, err)
					return
				}
			}
		}(h)
	}

	for _, h := range hooksToAdd {
		wg.Add(1)
		go func(wh hookItem) {
			defer wg.Done()
			ch, err := systemconfig.New().GetCodeHost(wh.codeHostID)
			if err != nil {
				logger.Errorf("Failed to get codeHost by id %d, err: %s", wh.codeHostID, err)
				errs = multierror.Append(errs, err)
				return
			}

			switch ch.Type {
			case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
				err = webhook.NewClient().AddWebHook(&webhook.TaskOption{
					ID:        ch.ID,
					Name:      wh.name,
					Owner:     wh.owner,
					Namespace: wh.namespace,
					Repo:      wh.repo,
					Address:   ch.Address,
					Token:     ch.AccessToken,
					Ref:       name,
					AK:        ch.AccessKey,
					SK:        ch.SecretKey,
					Region:    ch.Region,
					From:      ch.Type,
					IsManual:  wh.IsManual,
				})
				if err != nil {
					logger.Errorf("Failed to add %s webhook %+v, err: %s", ch.Type, wh, err)
					errs = multierror.Append(errs, err)
					return
				}
			}
		}(h)
	}

	wg.Wait()

	return errs.ErrorOrNil()
}

func toHookSet(hooks interface{}) HookSet {
	res := NewHookSet()
	switch hs := hooks.(type) {
	case []*models.WorkflowHook:
		// deprecated, for old custom workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.MainRepo.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
					source:    h.MainRepo.Source,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []models.GitHook:
		// for pipline workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.Owner,
					namespace: h.Owner, // webhooks for pipelines, no need to handler anymore
					repo:      h.Repo,
				},
				codeHostID: h.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*webhook.WebHook:
		// for template service sync
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.Owner,
					namespace: h.Namespace,
					repo:      h.Repo,
				},
				codeHostID: h.CodeHostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.TestingHook:
		// for testing
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.MainRepo.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.ScanningHook:
		// for scanning
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:  h.RepoName,
					owner: h.RepoOwner,
					repo:  h.RepoName,
				},
				codeHostID: h.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.WorkflowV4GitHook:
		// for custom workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
					source:    h.MainRepo.Source,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	}

	return res
}
