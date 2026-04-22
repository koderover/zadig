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
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
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
	start := time.Now()
	token := req.Header.Get("X-Gitlab-Token")
	secret := util.GetGitHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := gitlab.HookEventType(req)
	parseStart := time.Now()
	event, err := gitlab.ParseHook(eventType, payload)
	if err != nil {
		return err
	}
	log.Infof("gitlab webhook parsed event type %s in %s", eventType, time.Since(parseStart))

	baseURI := config.SystemAddress()
	var pushEvent *gitlab.PushEvent
	var mergeEvent *gitlab.MergeEvent
	var tagEvent *gitlab.TagEvent
	var errorList = &multierror.Error{}

	switch event.(type) {
	case *gitlab.PushSystemEvent:
		parsePushSystemEventStart := time.Now()
		if ev, err := gitlab.ParseWebhook(gitlab.EventTypePush, payload); err != nil {
			errorList = multierror.Append(errorList, err)
		} else {
			event = ev
			eventType = gitlab.EventTypePush
		}
		log.Infof("gitlab webhook parsed push system event in %s", time.Since(parsePushSystemEventStart))
	case *gitlab.MergeEvent:
		if eventType == gitlab.EventTypeSystemHook {
			eventType = gitlab.EventTypeMergeRequest
		}
	}

	switch event := event.(type) {
	case *gitlab.PushEvent:
		pushEvent = event
		pushEventStart := time.Now()
		changeFiles := make([]string, 0)
		for _, commit := range pushEvent.Commits {
			changeFiles = append(changeFiles, commit.Added...)
			changeFiles = append(changeFiles, commit.Removed...)
			changeFiles = append(changeFiles, commit.Modified...)
		}
		log.Infof("gitlab webhook collected %d changed files in %s", len(changeFiles), time.Since(pushEventStart))
		pathWithNamespace := pushEvent.Project.PathWithNamespace
		// trigger service template to re-sync from remote repo
		serviceSyncStart := time.Now()
		if err = updateServiceTemplateByPushEvent(pushEvent.Ref, changeFiles, pathWithNamespace, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
		log.Infof("gitlab webhook updateServiceTemplateByPushEvent cost %s", time.Since(serviceSyncStart))
		valuesSyncStart := time.Now()
		if err = updateServiceTemplateValuesByPushEvent(pushEvent.Ref, changeFiles, pathWithNamespace, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
		log.Infof("gitlab webhook updateServiceTemplateValuesByPushEvent cost %s", time.Since(valuesSyncStart))
		yamlSyncStart := time.Now()
		if err = updateYamlTemplateByGitlabPush(pushEvent.Ref, changeFiles, pathWithNamespace, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
		log.Infof("gitlab webhook updateYamlTemplateByGitlabPush cost %s", time.Since(yamlSyncStart))
	case *gitlab.MergeEvent:
		mergeEvent = event
	case *gitlab.TagEvent:
		tagEvent = event
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

		//测试管理webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerTestStart := time.Now()
			if err = TriggerTestByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerTestByGitlabEvent push cost %s", time.Since(triggerTestStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerScanningStart := time.Now()
			if err = TriggerScanningByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerScanningByGitlabEvent push cost %s", time.Since(triggerScanningStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerWorkflowV4Start := time.Now()
			if err = TriggerWorkflowV4ByGitlabEvent(pushEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerWorkflowV4ByGitlabEvent push cost %s", time.Since(triggerWorkflowV4Start))
		}()
	}

	if mergeEvent != nil && !shouldIgnoreGitlabMRWebhook(mergeEvent) {
		//测试管理webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerTestStart := time.Now()
			if err = TriggerTestByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerTestByGitlabEvent merge cost %s", time.Since(triggerTestStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerScanningStart := time.Now()
			if err = TriggerScanningByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerScanningByGitlabEvent merge cost %s", time.Since(triggerScanningStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerWorkflowV4Start := time.Now()
			if err = TriggerWorkflowV4ByGitlabEvent(mergeEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerWorkflowV4ByGitlabEvent merge cost %s", time.Since(triggerWorkflowV4Start))
		}()
	}

	if tagEvent != nil {
		//test webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerTestStart := time.Now()
			if err = TriggerTestByGitlabEvent(tagEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerTestByGitlabEvent tag cost %s", time.Since(triggerTestStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerScanningStart := time.Now()
			if err = TriggerScanningByGitlabEvent(tagEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerScanningByGitlabEvent tag cost %s", time.Since(triggerScanningStart))
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			triggerWorkflowV4Start := time.Now()
			if err = TriggerWorkflowV4ByGitlabEvent(tagEvent, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
			log.Infof("gitlab webhook TriggerWorkflowV4ByGitlabEvent tag cost %s", time.Since(triggerWorkflowV4Start))
		}()
	}

	waitStart := time.Now()
	wg.Wait()
	log.Infof("gitlab webhook wait async tasks cost %s", time.Since(waitStart))
	log.Infof("gitlab webhook total cost %s", time.Since(start))

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

func updateServiceTemplateByPushEvent(ref string, diffs []string, pathWithNamespace string, log *zap.SugaredLogger) error {
	log.Infof("EVENT: GITLAB WEBHOOK UPDATING SERVICE TEMPLATE")

	svcTmplsMap := map[bool][]*commonmodels.Service{}
	serviceTmpls, err := GetGitlabTestingServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get gitlab testing service templates, error: %v", err)
		return err
	}
	svcTmplsMap[false] = serviceTmpls
	productionServiceTmpls, err := GetGitlabProductionServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get gitlab production service templates, error: %v", err)
		return err
	}
	svcTmplsMap[true] = productionServiceTmpls

	errs := &multierror.Error{}
	for production, serviceTmpls := range svcTmplsMap {
		for _, service := range serviceTmpls {
			if service.Source != setting.SourceFromGitlab {
				continue
			}
			if service.GetRepoNamespace()+"/"+service.RepoName != pathWithNamespace {
				continue
			}

			if !checkBranchMatch(service, production, ref, log) {
				continue
			}

			path, err := getServiceSrcPath(service)
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			// 判断PushEvent的Diffs中是否包含该服务模板的src_path
			affected := false
			for _, diff := range diffs {
				if subElem(path, diff) {
					affected = true
					break
				}
			}
			if affected {
				log.Infof("Started to sync service template %s from gitlab %s, production: %v", service.ServiceName, service.SrcPath, production)
				//TODO: 异步处理
				service.CreateBy = "system"
				service.Production = production
				err := SyncServiceTemplateFromGitlab(service, log)
				if err != nil {
					log.Errorf("SyncServiceTemplateFromGitlab failed, error: %v", err)
					errs = multierror.Append(errs, err)
				}
			} else {
				log.Infof("Service template %s from gitlab %s is not affected, no sync", service.ServiceName, service.SrcPath)
			}
		}
	}

	return errs.ErrorOrNil()
}

func updateYamlTemplateByGitlabPush(ref string, diffs []string, pathWithNamespace string, log *zap.SugaredLogger) error {
	start := time.Now()
	templates, err := commonrepo.NewYamlTemplateColl().ListBySource(setting.SourceFromGitlab)
	if err != nil {
		return err
	}

	errs := &multierror.Error{}
	for _, tmpl := range templates {
		if tmpl == nil || tmpl.Source != setting.SourceFromGitlab {
			continue
		}
		namespace := tmpl.Namespace
		if namespace == "" {
			namespace = tmpl.RepoOwner
		}
		if namespace+"/"+tmpl.RepoName != pathWithNamespace {
			continue
		}
		if strings.TrimPrefix(ref, "refs/heads/") != tmpl.BranchName {
			continue
		}

		affected := len(diffs) == 0
		for _, diff := range diffs {
			if subElem(tmpl.Path, diff) {
				affected = true
				break
			}
		}
		if affected {
			log.Infof("Started to sync yaml template %s from gitlab path %s", tmpl.Name, tmpl.Path)
			if err := SyncYamlTemplateFromGitlab(tmpl, log); err != nil {
				log.Errorf("failed to sync yaml template %s from gitlab, error: %v", tmpl.Name, err)
				errs = multierror.Append(errs, err)
			}
		} else {
			log.Infof("Yaml template %s from gitlab %s is not affected, no sync", tmpl.Name, tmpl.Path)
		}
	}

	log.Infof("gitlab webhook updateYamlTemplateByGitlabPush scanned %d templates in %s", len(templates), time.Since(start))
	return errs.ErrorOrNil()
}

func SyncYamlTemplateFromGitlab(tmpl *commonmodels.YamlTemplate, log *zap.SugaredLogger) error {
	start := time.Now()
	if tmpl.Source != setting.SourceFromGitlab {
		return fmt.Errorf("yaml template is not from gitlab")
	}

	var before string
	if tmpl.Commit != nil {
		before = tmpl.Commit.SHA
	}

	client, err := getGitlabClientByCodehostId(tmpl.CodeHostID)
	if err != nil {
		return err
	}

	namespace := tmpl.Namespace
	if namespace == "" {
		namespace = tmpl.RepoOwner
	}

	latestCommitStart := time.Now()
	commit, err := GitlabGetLatestCommit(client, namespace, tmpl.RepoName, tmpl.BranchName, tmpl.Path)
	if err != nil {
		return err
	}
	log.Infof("gitlab webhook sync yaml template %s get latest commit cost %s", tmpl.Name, time.Since(latestCommitStart))

	tmpl.Commit = &commonmodels.Commit{
		SHA:     commit.ID,
		Message: commit.Message,
	}

	if before == tmpl.Commit.SHA {
		log.Infof("Before and after SHA: %s remains the same, no need to sync", before)
		return nil
	}

	pathType := "blob"
	if tmpl.LoadFromDir {
		pathType = "tree"
	}
	rawFilesStart := time.Now()
	files, err := GitlabGetRawFiles(client, namespace, tmpl.RepoName, tmpl.BranchName, tmpl.Path, pathType)
	if err != nil {
		return err
	}
	log.Infof("gitlab webhook sync yaml template %s get raw files cost %s", tmpl.Name, time.Since(rawFilesStart))
	if len(files) == 0 {
		return fmt.Errorf("no yaml file is found under directory %s", tmpl.Path)
	}

	content := util.JoinYamls(files)
	if pathType == "blob" {
		content = files[0]
	}

	updateTemplateStart := time.Now()
	if err := templateservice.UpdateYamlTemplate(tmpl.ID.Hex(), &template.YamlTemplate{
		Name:        tmpl.Name,
		Content:     content,
		Source:      tmpl.Source,
		CodehostID:  tmpl.CodeHostID,
		RepoOwner:   tmpl.RepoOwner,
		Namespace:   tmpl.Namespace,
		RepoName:    tmpl.RepoName,
		Path:        tmpl.Path,
		BranchName:  tmpl.BranchName,
		RemoteName:  tmpl.RemoteName,
		LoadFromDir: tmpl.LoadFromDir,
		Commit:      tmpl.Commit,
	}, log); err != nil {
		return err
	}
	log.Infof("gitlab webhook sync yaml template %s update template cost %s", tmpl.Name, time.Since(updateTemplateStart))

	log.Infof("End of sync yaml template %s from gitlab path %s, total cost %s", tmpl.Name, tmpl.Path, time.Since(start))
	return nil
}

func GetGitlabTestingServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGitlab,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

func GetGitlabProductionServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGitlab,
	}
	return commonrepo.NewProductionServiceColl().ListMaxRevisions(opt)
}

func GetHelmChartTemplateServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Type:   setting.HelmDeployType,
		Source: setting.SourceFromChartTemplate,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

func GetHelmChartTemplateProductionServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Type:   setting.HelmDeployType,
		Source: setting.SourceFromChartTemplate,
	}
	return commonrepo.NewProductionServiceColl().ListMaxRevisions(opt)
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
		log.Errorf("fillServiceTmpl error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s from gitlab path %s", service.ServiceName, service.SrcPath)
	return nil
}

// SyncServiceTemplateValuesFromGitlab Force to sync Service Template Values to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateValuesFromGitlab(service *commonmodels.Service, log *zap.SugaredLogger) error {
	sourceFrom, err := service.GetHelmValuesSourceRepo()
	if err != nil {
		return fmt.Errorf("service %s's helm values create_from is invalid", service.ServiceName)
	}

	err = checkCodeHostIsGitlab(sourceFrom.GitRepoConfig.CodehostID)
	if err != nil {
		return err
	}

	// 获取当前Commit的SHA
	var before string
	if sourceFrom.Commit != nil {
		before = sourceFrom.Commit.SHA
	}
	// Sync最新的Commit的SHA
	var after string
	err = syncValuesLatestCommit(sourceFrom)
	if err != nil {
		return err
	}
	after = sourceFrom.Commit.SHA
	// 判断一下是否需要Sync内容
	if before == after {
		log.Infof("Before and after SHA: %s remains the same, no need to sync", before)
		// 无需更新
		return nil
	}
	// 在Ensure过程中会检查source，如果source为gitlab，则同步gitlab内容到service中
	if err := fillServiceTmplValues(setting.WebhookTaskCreator, service, log); err != nil {
		log.Errorf("fillServiceTmpl error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s's values from gitlab path %s", service.ServiceName, sourceFrom.LoadPath)
	return nil
}

func updateServiceTemplateValuesByPushEvent(ref string, diffs []string, pathWithNamespace string, log *zap.SugaredLogger) error {
	log.Infof("EVENT: GITLAB WEBHOOK UPDATING SERVICE TEMPLATE VALUES")

	svcTmplsMap := map[bool][]*commonmodels.Service{}
	serviceTmpls, err := GetHelmChartTemplateServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get gitlab testing service templates, error: %v", err)
		return err
	}
	svcTmplsMap[false] = serviceTmpls
	productionServiceTmpls, err := GetHelmChartTemplateProductionServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get gitlab production service templates, error: %v", err)
		return err
	}
	svcTmplsMap[true] = productionServiceTmpls

	errs := &multierror.Error{}
	for production, serviceTmpls := range svcTmplsMap {
		for _, service := range serviceTmpls {
			if service.CreateFrom == nil {
				continue
			}

			createFrom, err := service.GetHelmCreateFrom()
			if err != nil {
				log.Errorf("Failed to get helm create from, error: %v", err)
				continue
			}

			sourceRepo, err := createFrom.GetSourceDetail()
			if err != nil {
				log.Errorf("Failed to get source detail, error: %v", err)
				continue
			}

			if sourceRepo.GitRepoConfig == nil {
				continue
			}

			if sourceRepo.GitRepoConfig.GetNamespace()+"/"+sourceRepo.GitRepoConfig.Repo != pathWithNamespace {
				continue
			}

			if !checkBranchMatch(service, production, ref, log) {
				continue
			}

			// 判断PushEvent的Diffs中是否包含该服务模板的LoadPath
			affected := false
			path := sourceRepo.LoadPath
			for _, diff := range diffs {
				if subElem(path, diff) {
					affected = true
					break
				}
			}
			if affected {
				log.Infof("Started to sync service template %s's values from gitlab %s, production: %v", service.ServiceName, sourceRepo.LoadPath, production)
				//TODO: 异步处理
				service.CreateBy = "system"
				service.Production = production
				err := SyncServiceTemplateValuesFromGitlab(service, log)
				if err != nil {
					log.Errorf("SyncServiceTemplateValuesFromGitlab failed, error: %v", err)
					errs = multierror.Append(errs, err)
				}
			} else {
				log.Infof("Service template %s's values from gitlab %s is not affected, no sync, production %v", service.ServiceName, sourceRepo.LoadPath, production)
			}
		}
	}

	return errs.ErrorOrNil()
}

func shouldIgnoreGitlabMRWebhook(ev *gitlab.MergeEvent) bool {
	if ev == nil {
		return false
	}

	// Only filter update events. Newly opened merge requests still need to flow
	// through the existing MR trigger path.
	if ev.ObjectAttributes.Action != "update" {
		log.Debugf("skip ignoring gitlab mr webhook: iid=%d action=%s", ev.ObjectAttributes.IID, ev.ObjectAttributes.Action)
		return false
	}

	// GitLab sets oldrev on update events that carry actual code changes,
	// such as new commits pushed to the source branch.
	ignored := ev.ObjectAttributes.OldRev == ""
	log.Debugf("evaluate gitlab mr webhook ignore: iid=%d action=%s has_oldrev=%t ignored=%t", ev.ObjectAttributes.IID, ev.ObjectAttributes.Action, ev.ObjectAttributes.OldRev != "", ignored)
	return ignored
}
