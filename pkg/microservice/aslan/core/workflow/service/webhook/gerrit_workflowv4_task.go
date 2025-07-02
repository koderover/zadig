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
	"regexp"
	"strconv"
	"strings"

	"github.com/coocood/freecache"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gerritEventMatcherForWorkflowV4 interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository
}

type patchsetCreatedEvent struct {
	Uploader       UploaderInfo  `json:"uploader"`
	PatchSet       PatchSetInfo  `json:"patchSet"`
	Change         ChangeInfo    `json:"change"`
	Project        ProjectInfo   `json:"project"`
	RefName        string        `json:"refName"`
	ChangeKey      ChangeKeyInfo `json:"changeKey"`
	Type           string        `json:"type"`
	EventCreatedOn int           `json:"eventCreatedOn"`
}

type PatchSetInfo struct {
	Number         int          `json:"number"`
	Revision       string       `json:"revision"`
	Parents        []string     `json:"parents"`
	Ref            string       `json:"ref"`
	Uploader       UploaderInfo `json:"uploader"`
	CreatedOn      int          `json:"createdOn"`
	Author         AuthorInfo   `json:"author"`
	Kind           string       `json:"kind"`
	SizeInsertions int          `json:"sizeInsertions"`
	SizeDeletions  int          `json:"sizeDeletions"`
}

type ChangeInfo struct {
	Project       string    `json:"project"`
	Branch        string    `json:"branch"`
	ID            string    `json:"id"`
	Number        int       `json:"number"`
	Subject       string    `json:"subject"`
	Owner         OwnerInfo `json:"owner"`
	URL           string    `json:"url"`
	CommitMessage string    `json:"commitMessage"`
	CreatedOn     int       `json:"createdOn"`
	Status        string    `json:"status"`
}

type UploaderInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type AuthorInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type OwnerInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type ProjectInfo struct {
	Name string `json:"name"`
}

type ChangeKeyInfo struct {
	ID string `json:"id"`
}

type changeMergedEvent struct {
	Submitter      SubmitterInfo `json:"submitter"`
	NewRev         string        `json:"newRev"`
	PatchSet       PatchSetInfo  `json:"patchSet"`
	Change         ChangeInfo    `json:"change"`
	Project        ProjectInfo   `json:"project"`
	RefName        string        `json:"refName"`
	ChangeKey      ChangeKeyInfo `json:"changeKey"`
	Type           string        `json:"type"`
	EventCreatedOn int           `json:"eventCreatedOn"`
}

type SubmitterInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type gerritChangeMergedEventMatcherForWorkflowV4 struct {
	Log      *zap.SugaredLogger
	Item     *commonmodels.WorkflowV4GitHook
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
	Item     *commonmodels.WorkflowV4GitHook
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

func createGerritEventMatcherForWorkflowV4(event *gerritTypeEvent, body []byte, item *commonmodels.WorkflowV4GitHook, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger) gerritEventMatcherForWorkflowV4 {
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
		gitHooks, err := commonrepo.NewWorkflowV4GitHookColl().List(internalhandler.NewBackgroupContext(), workflow.Name)
		if err != nil {
			log.Errorf("list workflow v4 git hook error: %v", err)
			return fmt.Errorf("list workflow v4 git hook error: %v", err)
		}
		if len(gitHooks) == 0 {
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
		for _, item := range gitHooks {
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
					// TODO THE OLD IMPLEMENTATION DOES NOT WORK FOR A LONG TIME, SO I DELETED THEM. REWITE IT WHEN NECESSARY.
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

var cache = freecache.NewCache(1024 * 1024 * 10)

func dealMultiTrigger(event *gerritTypeEvent, body []byte, workflowName string, log *zap.SugaredLogger) bool {
	if event.Type != patchsetCreatedEventType {
		return false
	}

	var ev patchsetCreatedEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		log.Errorf("createGerritEventMatcher json.Unmarshal err : %v", err)
	}
	mergeRequestID := strconv.Itoa(ev.Change.Number)
	commitID := strconv.Itoa(ev.PatchSet.Number)
	if mergeRequestID == "" || commitID == "" || workflowName == "" {
		log.Warnf("patchsetCreatedEvent param cannot be empty, mergeRequestID:%s, commitID:%s,workflowName:%s", mergeRequestID, commitID, workflowName)
		return true
	}

	keyStr := fmt.Sprintf("%s+%s+%s+%s", setting.SourceFromGerrit, mergeRequestID, commitID, workflowName)
	key := []byte(keyStr)
	_, err := cache.Get(key)
	if err != nil {
		// 找不到则插入新key
		log.Infof("find key in freecache failed, err:%v\n", err)
		val := []byte("true")
		expire := 180 // expire in 180 seconds
		err = cache.Set(key, val, expire)
		if err != nil {
			return false
		}
		return false
	}
	return true
}
