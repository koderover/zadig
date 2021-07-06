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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collie"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type EventPush struct {
	EventName   string `json:"event_name"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Ref         string `json:"ref"`
	CheckoutSha string `json:"checkout_sha"`
	ProjectID   int    `json:"project_id"`
	Body        string `json:"body"`
}

func ProcessGitlabHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-Gitlab-Token")
	secret := gitservice.GetHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := gitlab.HookEventType(req)
	event, err := gitlab.ParseHook(eventType, payload)
	if err != nil {
		return err
	}

	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseURI := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	var eventPush *EventPush
	var pushEvent *gitlab.PushEvent
	var mergeEvent *gitlab.MergeEvent
	var errorList = &multierror.Error{}

	switch event.(type) {
	case *gitlab.PushSystemEvent:
		if ev, err := gitlab.ParseWebhook(gitlab.EventTypePush, payload); err != nil {
			errorList = multierror.Append(errorList, err)
		} else {
			event = ev
			eventType = gitlab.EventTypePush
		}
	case *gitlab.MergeEvent:
		if eventType == gitlab.EventTypeSystemHook {
			eventType = gitlab.EventTypeMergeRequest
		}
	}

	go collie.CallGitlabWebHook(forwardedProto, forwardedHost, payload, string(eventType), log)

	switch event := event.(type) {
	case *gitlab.PushEvent:
		eventPush = &EventPush{
			Before:      event.Before,
			After:       event.After,
			Ref:         event.Ref,
			CheckoutSha: event.CheckoutSHA,
			ProjectID:   event.ProjectID,
			Body:        string(payload),
		}
		pushEvent = event
	case *gitlab.MergeEvent:
		mergeEvent = event
	}
	//触发更新服务模板webhook
	if eventPush != nil {
		if err = updateServiceTemplateByPushEvent(eventPush, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	//触发工作流webhook和测试管理webhook
	var wg sync.WaitGroup

	if pushEvent != nil {
		//add webhook user
		if len(pushEvent.Commits) > 0 {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  pushEvent.Commits[0].Author.Name,
				Email:     pushEvent.Commits[0].Author.Email,
				Source:    setting.SourceFromGitlab,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}

		//产品工作流webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerWorkflowByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()

		//单服务工作流webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerPipelineByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()

		//测试管理webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerTestByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
	}

	if mergeEvent != nil {
		//多服务工作流webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerWorkflowByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()

		//单服务工作流webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerPipelineByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()

		//测试管理webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerTestByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
	}

	wg.Wait()

	return errorList.ErrorOrNil()
}

type GitlabEvent struct {
	ObjectKind        string         `json:"object_kind"`
	EventName         string         `json:"event_name"`
	Before            string         `json:"before"`
	After             string         `json:"after"`
	Ref               string         `json:"ref"`
	CheckoutSha       string         `json:"checkout_sha"`
	Message           interface{}    `json:"message"`
	UserID            int            `json:"user_id"`
	UserName          string         `json:"user_name"`
	UserUsername      string         `json:"user_username"`
	UserEmail         string         `json:"user_email"`
	UserAvatar        string         `json:"user_avatar"`
	ProjectID         int            `json:"project_id"`
	Project           ProjectDetail  `json:"project"`
	Commits           []CommitInfo   `json:"commits"`
	TotalCommitsCount int            `json:"total_commits_count"`
	Repository        RepositoryInfo `json:"repository"`
}

type ProjectDetail struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type CommitInfo struct {
	ID        string        `json:"id"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	URL       string        `json:"url"`
	Author    AuthorInfo    `json:"author"`
	Added     []interface{} `json:"added"`
	Modified  []string      `json:"modified"`
	Removed   []interface{} `json:"removed"`
}

type RepositoryInfo struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	Homepage        string `json:"homepage"`
	GitHTTPURL      string `json:"git_http_url"`
	GitSSHURL       string `json:"git_ssh_url"`
	VisibilityLevel int    `json:"visibility_level"`
}

func updateServiceTemplateByPushEvent(event *EventPush, log *zap.SugaredLogger) error {
	log.Infof("EVENT: GITLAB WEBHOOK UPDATING SERVICE TEMPLATE")
	gitlabEvent := &GitlabEvent{}

	err := json.Unmarshal([]byte(event.Body), gitlabEvent)
	if err != nil {
		log.Errorf("Get Project ID failed, error: %v", err)
		return err
	}

	address, err := GetGitlabAddress(gitlabEvent.Project.WebURL)
	if err != nil {
		log.Errorf("GetGitlabAddress failed, error: %v", err)
		return err
	}

	client, err := getGitlabClientByAddress(address)
	if err != nil {
		return err
	}

	diffs, err := client.Compare(event.ProjectID, event.Before, event.After)
	if err != nil {
		log.Errorf("Failed to get push event diffs, error: %v", err)
		return err
	}
	serviceTmpls, err := GetGitlabServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get gitlab service templates, error: %v", err)
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
			log.Infof("Started to sync service template %s from gitlab %s", service.ServiceName, service.SrcPath)
			//TODO: 异步处理
			service.CreateBy = "system"
			err := SyncServiceTemplateFromGitlab(service, log)
			if err != nil {
				log.Errorf("SyncServiceTemplateFromGitlab failed, error: %v", err)
				errs = multierror.Append(errs, err)
			}
		} else {
			log.Infof("Service template %s from gitlab %s is not affected, no sync", service.ServiceName, service.SrcPath)
		}

	}
	return errs.ErrorOrNil()
}

func GetGitlabServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceFindOption{
		Type:          setting.K8SDeployType,
		Source:        setting.SourceFromGitlab,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	return commonrepo.NewServiceColl().List(opt)
}

// SyncServiceTemplateFromGitlab Force to sync Service Template to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromGitlab(service *commonmodels.Service, log *zap.SugaredLogger) error {
	// 判断一下Source字段，如果Source字段不是gitlab，直接返回
	if service.Source != setting.SourceFromGitlab {
		return fmt.Errorf("service template is not from gitlab")
	}
	// 获取当前Commit的SHA
	var before string
	if service.Commit != nil {
		before = service.Commit.SHA
	}
	// Sync最新的Commit的SHA
	var after string
	err := syncLatestCommit(service)
	if err != nil {
		return err
	}
	after = service.Commit.SHA
	// 判断一下是否需要Sync内容
	if before == after {
		log.Infof("Before and after SHA: %s remains the same, no need to sync", before)
		// 无需更新
		return nil
	}
	// 在Ensure过程中会检查source，如果source为gitlab，则同步gitlab内容到service中
	if err := fillServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}
	// 更新到数据库，revision+1
	if err := commonrepo.NewServiceColl().Create(service); err != nil {
		log.Errorf("Failed to sync service %s from gitlab path %s error: %v", service.ServiceName, service.SrcPath, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s from gitlab path %s", service.ServiceName, service.SrcPath)
	return nil
}
