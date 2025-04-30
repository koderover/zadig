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

package webhook

import (
	"encoding/json"
	"fmt"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gerritEventMatcherForWorkflowV4 interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository
}

type gerritChangeMergedEventMatcherForWorkflowV4 struct {
	Log      *zap.SugaredLogger
	Item     *commonmodels.WorkflowV4Hook
	Workflow *commonmodels.WorkflowV4
	Event    *changeMergedEvent
}

func (gruem *gerritChangeMergedEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	event := gruem.Event
	if event == nil {
		return false, fmt.Errorf("event doesn't match")
	}

	if event.Project.Name == gruem.Item.MainRepo.RepoName {
		refName := getBranchFromRef(event.RefName)
		isRegular := gruem.Item.MainRepo.IsRegular
		if !isRegular && hookRepo.Branch != refName {
			return false, nil
		}
		if isRegular {
			// Do not use regexp.MustCompile to avoid panic
			matched, err := regexp.MatchString(gruem.Item.MainRepo.Branch, refName)
			if err != nil || !matched {
				return false, nil
			}
		}
		hookRepo.Branch = refName
		existEventNames := make([]string, 0)
		for _, eventName := range gruem.Item.MainRepo.Events {
			existEventNames = append(existEventNames, string(eventName))
		}
		if sets.NewString(existEventNames...).Has(event.Type) {
			hookRepo.Committer = event.Submitter.Username
			return true, nil
		}
	}
	return false, nil
}

func (gruem *gerritChangeMergedEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
	}
}

type gerritPatchsetCreatedEventMatcherForWorkflowV4 struct {
	Log      *zap.SugaredLogger
	Item     *commonmodels.WorkflowV4Hook
	Workflow *commonmodels.WorkflowV4
	Event    *patchsetCreatedEvent
}

func (gpcem *gerritPatchsetCreatedEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	event := gpcem.Event
	if event == nil {
		return false, fmt.Errorf("event doesn't match")
	}

	if event.Project.Name == gpcem.Item.MainRepo.RepoName {
		refName := getBranchFromRef(event.RefName)
		isRegular := gpcem.Item.MainRepo.IsRegular
		if !isRegular && hookRepo.Branch != refName {
			return false, nil
		}
		if isRegular {
			// Do not use regexp.MustCompile to avoid panic
			matched, err := regexp.MatchString(gpcem.Item.MainRepo.Branch, refName)
			if err != nil || !matched {
				return false, nil
			}
		}
		hookRepo.Branch = refName
		existEventNames := make([]string, 0)
		for _, eventName := range gpcem.Item.MainRepo.Events {
			existEventNames = append(existEventNames, string(eventName))
		}
		if sets.NewString(existEventNames...).Has(event.Type) {
			hookRepo.Committer = event.Uploader.Username
			return true, nil
		}
	}
	return false, nil
}

func (gpcem *gerritPatchsetCreatedEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            gpcem.Event.Change.Number,
		Source:        hookRepo.Source,
	}
}

func createGerritEventMatcherForWorkflowV4(event *gerritTypeEvent, body []byte, item *commonmodels.WorkflowV4Hook, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger) gerritEventMatcherForWorkflowV4 {
	switch event.Type {
	case changeMergedEventType:
		changeMergedEvent := new(changeMergedEvent)
		if err := json.Unmarshal(body, changeMergedEvent); err != nil {
			log.Errorf("createGerritEventMatcher json.Unmarshal err : %v", err)
		}
		return &gerritChangeMergedEventMatcherForWorkflowV4{
			Workflow: workflow,
			Item:     item,
			Log:      log,
			Event:    changeMergedEvent,
		}
	case patchsetCreatedEventType:
		var ev patchsetCreatedEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			log.Errorf("createGerritEventMatcher json.Unmarshal err : %v", err)
		}
		return &gerritPatchsetCreatedEventMatcherForWorkflowV4{
			Workflow: workflow,
			Item:     item,
			Log:      log,
			Event:    &ev,
		}
	}

	return nil
}

func TriggerWorkflowV4ByGerritEvent(event *gerritTypeEvent, body []byte, uri, baseURI, domain, requestID string, log *zap.SugaredLogger) error {
	log.Infof("gerrit webhook request url:%s\n", uri)

	workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{}, 0, 0)
	if err != nil {
		errMsg := fmt.Sprintf("list workflow v4 error: %v", err)
		log.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	var errorList = &multierror.Error{}
	var hookPayload *commonmodels.HookPayload
	var notification *commonmodels.Notification
	for _, workflow := range workflows {
		if workflow.HookCtls == nil {
			continue
		}
		if strings.Contains(uri, "?") {
			if !strings.Contains(uri, workflow.Name) {
				continue
			}
		}
		isMultiTrigger := dealMultiTrigger(event, body, workflow.Name, log)
		if isMultiTrigger {
			log.Warnf("this event has already trigger task, workflowName:%s, event:%v", workflow.Name, event)
			continue
		}
		for _, item := range workflow.HookCtls {
			if !item.Enabled {
				continue
			}

			if item.WorkflowArg == nil {
				continue
			}
			detail, err := systemconfig.New().GetCodeHost(item.MainRepo.CodehostID)
			if err != nil {
				log.Errorf("TriggerWorkflowByGerritEvent GetCodehostDetail err:%v", err)
				return err
			}
			if detail.Type != gerrit.CodehostTypeGerrit {
				continue
			}
			matcher := createGerritEventMatcherForWorkflowV4(event, body, item, workflow, log)
			if matcher == nil {
				continue
			}
			isMatch, err := matcher.Match(item.MainRepo)
			if err != nil {
				errorList = multierror.Append(errorList, err)
			}
			if !isMatch {
				continue
			}
			log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)

			eventRepo := matcher.GetHookRepo(item.MainRepo)

			var mergeRequestID, commitID string
			if m, ok := matcher.(*gerritPatchsetCreatedEventMatcherForWorkflowV4); ok {
				if item.CheckPatchSetChange {
					// for different patch sets under the same pr, if the updated contents of the two patch sets are exactly the same, and the task triggered by the previous patch set is executed successfully, the new patch set will no longer trigger the task.
					if checkLatestTaskStaus(workflow.Name, mergeRequestID, commitID, detail, log) {
						log.Infof("last patchset has already triggered task, workflowName:%s, mergeRequestID:%s, PatchSetID:%s", workflow.Name, mergeRequestID, commitID)
						continue
					}
				}

				mergeRequestID = strconv.Itoa(m.Event.Change.Number)
				commitID = strconv.Itoa(m.Event.PatchSet.Number)
				autoCancelOpt := &AutoCancelOpt{
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
					TaskType:       config.WorkflowType,
					MainRepo:       item.MainRepo,
					AutoCancel:     item.AutoCancel,
					WorkflowName:   workflow.Name,
				}
				err := AutoCancelWorkflowV4Task(autoCancelOpt, log)
				if err != nil {
					log.Errorf("failed to auto cancel workflowV4 task when receive event %v due to %v ", event, err)
					errorList = multierror.Append(errorList, err)
				}

				if notification == nil {
					// gerrit has no repo owner
					mainRepo := item.MainRepo
					mainRepo.RepoOwner = ""
					mainRepo.Revision = m.Event.PatchSet.Revision
					notification, _ = scmnotify.NewService().SendInitWebhookComment(
						mainRepo, m.Event.Change.Number, baseURI, false, false, false, true, log,
					)
				}

				hookPayload = &commonmodels.HookPayload{
					Owner:          eventRepo.RepoOwner,
					Repo:           eventRepo.RepoName,
					Branch:         eventRepo.Branch,
					IsPr:           true,
					CodehostID:     item.MainRepo.CodehostID,
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
				}
			}
			workflowController := controller.CreateWorkflowController(item.WorkflowArg)
			if err := workflowController.UpdateWithLatestWorkflow(nil); err != nil {
				errMsg := fmt.Sprintf("merge workflow args error: %v", err)
				log.Error(errMsg)
				errorList = multierror.Append(errorList, fmt.Errorf(errMsg))
				continue
			}
			if err := workflowController.SetRepo(eventRepo); err != nil {
				errMsg := fmt.Sprintf("merge webhook repo info to workflowargs error: %v", err)
				log.Error(errMsg)
				errorList = multierror.Append(errorList, fmt.Errorf(errMsg))
				continue
			}
			if notification != nil {
				workflowController.NotificationID = notification.ID.Hex()
			}
			workflowController.HookPayload = hookPayload
			if resp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
				Name: setting.WebhookTaskCreator,
			}, workflowController.WorkflowV4, log); err != nil {
				errMsg := fmt.Sprintf("failed to create workflow task when receive push event due to %v ", err)
				log.Error(errMsg)
				errorList = multierror.Append(errorList, fmt.Errorf(errMsg))
			} else {
				log.Infof("succeed to create task %v", resp)
			}

		}
	}
	return errorList.ErrorOrNil()
}
