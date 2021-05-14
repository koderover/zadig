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

package webhook

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coocood/freecache"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
	"github.com/koderover/zadig/lib/types/permission"
)

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

var cache = freecache.NewCache(1024 * 1024 * 10)

func dealMultiTrigger(event *gerritTypeEvent, body []byte, workflowName string, log *xlog.Logger) bool {
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

type gerritEventMatcher interface {
	Match(commonmodels.MainHookRepo) (bool, error)
	UpdateTaskArgs(*commonmodels.Product, *commonmodels.WorkflowTaskArgs, commonmodels.MainHookRepo) *commonmodels.WorkflowTaskArgs
}

type gerritChangeMergedEventMatcher struct {
	Log      *xlog.Logger
	Item     *commonmodels.WorkflowHook
	Workflow *commonmodels.Workflow
	Event    *changeMergedEvent
}

func (gruem *gerritChangeMergedEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	event := gruem.Event
	if event == nil {
		return false, fmt.Errorf("event doesn't match")
	}

	if event.Project.Name == gruem.Item.MainRepo.RepoName && strings.Contains(event.RefName, gruem.Item.MainRepo.Branch) {
		existEventNames := make([]string, 0)
		for _, eventName := range gruem.Item.MainRepo.Events {
			existEventNames = append(existEventNames, string(eventName))
		}
		if sets.NewString(existEventNames...).Has(event.Type) {
			return true, nil
		}
	}
	return false, nil
}

func (gruem *gerritChangeMergedEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gruem.Workflow,
		reqId:    gruem.Log.ReqID(),
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
	})

	return args
}

type gerritPatchsetCreatedEventMatcher struct {
	Log      *xlog.Logger
	Item     *commonmodels.WorkflowHook
	Workflow *commonmodels.Workflow
	Event    *patchsetCreatedEvent
}

func (gpcem *gerritPatchsetCreatedEventMatcher) Match(commonmodels.MainHookRepo) (bool, error) {
	event := gpcem.Event
	if event == nil {
		return false, fmt.Errorf("event doesn't match")
	}

	if event.Project.Name == gpcem.Item.MainRepo.RepoName && strings.Contains(event.RefName, gpcem.Item.MainRepo.Branch) {
		existEventNames := make([]string, 0)
		for _, eventName := range gpcem.Item.MainRepo.Events {
			existEventNames = append(existEventNames, string(eventName))
		}
		if sets.NewString(existEventNames...).Has(event.Type) {
			return true, nil
		}
	}
	return false, nil
}

func (gpcem *gerritPatchsetCreatedEventMatcher) UpdateTaskArgs(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gpcem.Workflow,
		reqId:    gpcem.Log.ReqID(),
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
		PR:         gpcem.Event.Change.Number,
	})

	return args
}

func createGerritEventMatcher(event *gerritTypeEvent, body []byte, item *commonmodels.WorkflowHook, workflow *commonmodels.Workflow, log *xlog.Logger) gerritEventMatcher {
	switch event.Type {
	case changeMergedEventType:
		changeMergedEvent := new(changeMergedEvent)
		if err := json.Unmarshal(body, changeMergedEvent); err != nil {
			log.Errorf("createGerritEventMatcher json.Unmarshal err : %v", err)
		}
		return &gerritChangeMergedEventMatcher{
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
		return &gerritPatchsetCreatedEventMatcher{
			Workflow: workflow,
			Item:     item,
			Log:      log,
			Event:    &ev,
		}
	}

	return nil
}

func TriggerWorkflowByGerritEvent(event *gerritTypeEvent, body []byte, uri, baseUri, domain string, log *xlog.Logger) error {
	log.Infof("gerrit webhook request url:%s\n", uri)

	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("TriggerWorkflowByGerritEvent failed to list workflow %v", err)
		return err
	}
	var errorList = &multierror.Error{}
	var notification *commonmodels.Notification

	for _, workflow := range workflowList {
		if workflow.HookCtl != nil && workflow.HookCtl.Enabled {
			if strings.Contains(uri, "?") {
				if !strings.Contains(uri, workflow.Name) {
					continue
				}
			}

			// 避免gerrit重试时导致一个commit触发多个工作流任务
			// key：TriggerBy.Source+TriggerBy.MergeRequestID+TriggerBy.CommitID+PipelineName（工作流名称）
			isMultiTrigger := dealMultiTrigger(event, body, workflow.Name, log)
			if isMultiTrigger {
				log.Warnf("this event has already trigger task, workflowName:%s, event:%v", workflow.Name, event)
				continue
			}

			for _, item := range workflow.HookCtl.Items {
				if item.WorkflowArgs == nil {
					continue
				}
				detail, err := codehost.GetCodehostDetail(item.MainRepo.CodehostID)
				if err != nil {
					log.Errorf("TriggerWorkflowByGerritEvent GetCodehostDetail err:%v", err)
					return err
				}
				if detail.Source == gerrit.CodehostTypeGerrit {
					log.Debugf("TriggerWorkflowByGerritEvent find gerrit hook in workflow %s", workflow.Name)
					matcher := createGerritEventMatcher(event, body, item, workflow, log)
					if matcher == nil {
						continue
					}
					if isMatch, err := matcher.Match(item.MainRepo); err != nil {
						errorList = multierror.Append(errorList, err)
					} else if isMatch {
						log.Infof("TriggerWorkflowByGerritEvent event match hook %v %v of %s", event, item.MainRepo, workflow.Name)
						namespace := strings.Split(item.WorkflowArgs.Namespace, ",")[0]
						opt := &commonrepo.ProductFindOptions{Name: workflow.ProductTmplName, EnvName: namespace}
						var prod *commonmodels.Product
						if prod, err = commonrepo.NewProductColl().Find(opt); err != nil {
							log.Warnf("TriggerWorkflowByGerritEvent can't find environment %s-%s", item.WorkflowArgs.Namespace, workflow.ProductTmplName)
							continue
						}

						// add webHook user
						addWebHookUser(matcher, domain)

						var mergeRequestID, commitID string
						if m, ok := matcher.(*gerritPatchsetCreatedEventMatcher); ok {
							mergeRequestID = strconv.Itoa(m.Event.Change.Number)
							commitID = strconv.Itoa(m.Event.PatchSet.Number)

							if item.CheckPatchSetChange {
								// 同一个pr下的不同patch set，如果两个patch set更新的内容完全相同，并且前一个patch set触发的任务执行成功，则新的patch set不再触发任务。
								if checkLatestTaskStaus(workflow.Name, mergeRequestID, commitID, detail, log) {
									log.Infof("last patchset has already triggered task, workflowName:%s, mergeRequestID:%s, PatchSetID:%s", workflow.Name, mergeRequestID, commitID)
									continue
								}
							}

							// 如果是merge request，且该webhook触发器配置了自动取消，
							// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
							autoCancelOpt := &AutoCancelOpt{
								MergeRequestID: mergeRequestID,
								CommitID:       commitID,
								TaskType:       config.WorkflowType,
								MainRepo:       item.MainRepo,
								WorkflowArgs:   item.WorkflowArgs,
							}
							err := AutoCancelTask(autoCancelOpt, log)
							if err != nil {
								log.Errorf("failed to auto cancel workflow task when receive event %v due to %v ", event, err)
								errorList = multierror.Append(errorList, err)
							}

							if notification == nil {
								// gerrit has no repo owner
								mainRepo := item.MainRepo
								mainRepo.RepoOwner = ""
								mainRepo.Revision = m.Event.PatchSet.Revision
								notification, _ = scmnotify.NewService().SendInitWebhookComment(
									&mainRepo, m.Event.Change.Number, baseUri, false, false, log,
								)
							}
						}

						workflowArgs := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo)
						if notification != nil {
							workflowArgs.NotificationId = notification.ID.Hex()
						}
						workflowArgs.MergeRequestID = mergeRequestID
						workflowArgs.CommitID = commitID
						workflowArgs.Source = setting.SourceFromGerrit
						workflowArgs.CodehostID = item.MainRepo.CodehostID
						workflowArgs.RepoOwner = item.MainRepo.RepoOwner
						workflowArgs.RepoName = item.MainRepo.RepoName

						if resp, err := workflowservice.CreateWorkflowTask(workflowArgs, setting.WebhookTaskCreator, permission.AnonymousUserID, false, log); err != nil {
							log.Errorf("TriggerWorkflowByGerritEvent failed to create workflow task when receive push event %v due to %v ", event, err)
							errorList = multierror.Append(errorList, err)
						} else {
							log.Infof("TriggerWorkflowByGerritEvent succeed to create task %v", resp)
						}
					}
				}
			}
		}
	}

	return errorList.ErrorOrNil()
}

//add webHook user
func addWebHookUser(match gerritEventMatcher, domain string) {
	if merge, ok := match.(*gerritChangeMergedEventMatcher); ok {
		webhookUser := &commonmodels.WebHookUser{
			Domain:    domain,
			UserName:  merge.Event.Change.Owner.Username,
			Email:     merge.Event.Change.Owner.Email,
			Source:    setting.SourceFromGerrit,
			CreatedAt: time.Now().Unix(),
		}
		commonrepo.NewWebHookUserColl().Upsert(webhookUser)
	}
}

func checkLatestTaskStaus(pipelineName, mergeRequestID, commitId string, detail *codehost.Detail, log *xlog.Logger) bool {
	opt := &commonrepo.ListTaskOption{
		PipelineName:   pipelineName,
		Type:           config.WorkflowType,
		TaskCreator:    setting.WebhookTaskCreator,
		Source:         setting.SourceFromGerrit,
		MergeRequestID: mergeRequestID,
		NeedTriggerBy:  true,
		Limit:          1,
	}
	tasks, err := commonrepo.NewTaskColl().List(opt)
	if err != nil {
		log.Errorf("PipelineTask.List failed, err:%v", err)
		return false
	}
	if len(tasks) == 0 {
		return false
	}

	if tasks[0].Status == config.StatusFailed || tasks[0].TriggerBy == nil {
		return false
	}

	// 比较本次patchset 和 上一个触发任务的patchset 的change file是否相同
	cli := gerrit.NewClient(detail.Address, detail.OauthToken)
	isDiff, err := cli.CompareTwoPatchset(mergeRequestID, commitId, tasks[0].TriggerBy.CommitID)
	if err != nil {
		log.Errorf("CompareTwoPatchset failed, mergeRequestID:%s, patchsetID:%s, oldPatchsetID:%s, err:%v", mergeRequestID, commitId, tasks[0].TriggerBy.CommitID, err)
		return false
	}
	if isDiff {
		return false
	}

	return true
}
