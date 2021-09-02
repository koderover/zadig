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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/ilyshin"
	"github.com/koderover/zadig/pkg/types/permission"
)

func ProcessIlyshinHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-CodeHub-Token")
	secret := gitservice.GetHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := ilyshin.HookEventType(req)
	event, err := ilyshin.ParseHook(eventType, payload)
	if err != nil {
		log.Warnf("unexpected event type: %s", eventType)
		return nil
	}

	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseURI := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	var pushEvent *ilyshin.PushEvent
	var errorList = &multierror.Error{}
	switch event := event.(type) {
	case *ilyshin.PushEvent:
		pushEvent = event
		if len(pushEvent.Commits) > 0 {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  pushEvent.Commits[0].Author.Name,
				Email:     pushEvent.Commits[0].Author.Email,
				Source:    setting.SourceFromIlyshin,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}

		if err = updateServiceTemplateByIlyshinPushEvent(pushEvent, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	//产品工作流webhook
	if err = TriggerWorkflowByIlyshinEvent(event, baseURI, requestID, log); err != nil {
		errorList = multierror.Append(errorList, err)
	}

	return errorList.ErrorOrNil()
}

func updateServiceTemplateByIlyshinPushEvent(event *ilyshin.PushEvent, log *zap.SugaredLogger) error {
	log.Infof("EVENT: Ilyshin WEBHOOK UPDATING SERVICE TEMPLATE")

	address, err := GetAddress(event.Project.WebURL)
	if err != nil {
		log.Errorf("GetAddress failed, error: %s", err)
		return err
	}

	client, err := getIlyshinClientByAddress(address)
	if err != nil {
		return err
	}

	diffs, err := client.Compare(event.ProjectID, event.Before, event.After)
	if err != nil {
		log.Errorf("Failed to get push event diffs, error: %s", err)
		return err
	}
	serviceTmpls, err := GetIlyshinServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get ilyshin service templates, error: %s", err)
		return err
	}

	errs := &multierror.Error{}
	for _, service := range serviceTmpls {
		srcPath := service.SrcPath
		_, _, _, _, path, _, err := GetOwnerRepoBranchPath(srcPath)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		// 判断PushEvent的Diffs中是否包含该服务模板的src_path
		affected := false
		for _, diff := range diffs {
			if strings.Contains(diff.OldPath, path) || strings.Contains(diff.NewPath, path) {
				affected = true
				break
			}
		}
		if affected {
			log.Infof("Started to sync service template %s from ilyshin %s", service.ServiceName, service.SrcPath)
			service.CreateBy = "system"
			err := SyncServiceTemplateFromIlyshin(service, log)
			if err != nil {
				log.Errorf("SyncServiceTemplateFromIlyshin failed, error: %s", err)
				errs = multierror.Append(errs, err)
			}
		} else {
			log.Infof("Service template %s from ilyshin %s is not affected, no sync", service.ServiceName, service.SrcPath)
		}

	}
	return errs.ErrorOrNil()
}

func GetIlyshinServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Type:   setting.K8SDeployType,
		Source: setting.SourceFromIlyshin,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

// SyncServiceTemplateFromIlyshin Force to sync Service Template to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromIlyshin(service *commonmodels.Service, log *zap.SugaredLogger) error {
	// 判断一下 Source 字段，如果 Source 字段不是 ilyshin，直接返回
	if service.Source != setting.SourceFromIlyshin {
		return fmt.Errorf("service template is not from ilyshin")
	}
	// 获取当前Commit的SHA
	var before string
	if service.Commit != nil {
		before = service.Commit.SHA
	}
	// Sync最新的Commit的SHA
	var after string
	err := syncIlyshinLatestCommit(service, log)
	if err != nil {
		return err
	}
	after = service.Commit.SHA
	// 判断一下是否需要 Sync 内容
	if before == after {
		log.Infof("Before and after SHA: %s remains the same, no need to sync", before)
		// 无需更新
		return nil
	}
	// 在 Ensure 过程中会检查 source，如果 source 为 ilyshin，则同步 ilyshin 内容到 service 中
	if err := fillServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}
	// 更新到数据库，revision+1
	if err := commonrepo.NewServiceColl().Create(service); err != nil {
		log.Errorf("Failed to sync service %s from ilyshin path %s error: %v", service.ServiceName, service.SrcPath, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s from ilyshin path %s", service.ServiceName, service.SrcPath)
	return nil
}

func TriggerWorkflowByIlyshinEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// 1. find configured workflow
	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("failed to list workflow %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(mergeEvent *ilyshin.MergeEvent, codehostId int) ([]string, error) {
		return findIlyshinChangedFilesOfMergeRequest(mergeEvent, codehostId)
	}

	for _, workflow := range workflowList {
		if workflow.HookCtl == nil || !workflow.HookCtl.Enabled {
			continue
		}

		log.Debugf("find %d hooks in workflow %s", len(workflow.HookCtl.Items), workflow.Name)
		for _, item := range workflow.HookCtl.Items {
			if item.WorkflowArgs == nil {
				continue
			}

			// 2. match webhook
			matcher := createIlyshinEventMatcher(event, diffSrv, workflow, log)
			if matcher == nil {
				continue
			}

			matches, err := matcher.Match(item.MainRepo)
			if err != nil {
				mErr = multierror.Append(mErr, err)
				continue
			}

			if !matches {
				log.Debugf("event not matches %v", item.MainRepo)
				continue
			}

			log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)
			namespace := strings.Split(item.WorkflowArgs.Namespace, ",")[0]
			opt := &commonrepo.ProductFindOptions{Name: workflow.ProductTmplName, EnvName: namespace}
			var prod *commonmodels.Product
			if prod, err = commonrepo.NewProductColl().Find(opt); err != nil {
				log.Warnf("can't find environment %s-%s", item.WorkflowArgs.Namespace, workflow.ProductTmplName)
				continue
			}

			var mergeRequestID, commitID string
			if ev, isPr := event.(*ilyshin.MergeEvent); isPr {
				// 如果是merge request，且该webhook触发器配置了自动取消，
				// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
				mergeRequestID = strconv.Itoa(ev.ObjectAttributes.IID)
				commitID = ev.ObjectAttributes.LastCommit.ID
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
					mErr = multierror.Append(mErr, err)
				}
			}

			args := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo, requestID)
			args.MergeRequestID = mergeRequestID
			args.CommitID = commitID
			args.Source = setting.SourceFromIlyshin
			args.CodehostID = item.MainRepo.CodehostID
			args.RepoOwner = item.MainRepo.RepoOwner
			args.RepoName = item.MainRepo.RepoName
			// 3. create task with args
			if _, err := workflowservice.CreateWorkflowTask(args, setting.WebhookTaskCreator, permission.AnonymousUserID, false, log); err != nil {
				log.Errorf("failed to create workflow task when receive push event %v due to %s ", event, err)
				mErr = multierror.Append(mErr, err)
			} else {
				log.Infof("succeed to create task")
			}
		}
	}

	return mErr.ErrorOrNil()
}

func findIlyshinChangedFilesOfMergeRequest(event *ilyshin.MergeEvent, codehostID int) ([]string, error) {
	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost %d: %s", codehostID, err)
	}

	client := ilyshin.NewClient(detail.Address, detail.OauthToken)
	return client.ListChangedFiles(event)
}
