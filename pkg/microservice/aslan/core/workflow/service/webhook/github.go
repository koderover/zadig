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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	signatureHeader  = "X-Hub-Signature"
	payloadFormParam = "payload"
)

func validateSecret(payload, secretKey []byte, r *http.Request) error {
	sig := r.Header.Get(signatureHeader)
	if len(secretKey) > 0 {
		return github.ValidateSignature(sig, payload, secretKey)
	}

	return nil
}

func prEventToPipelineTasks(event *github.PullRequestEvent, requestID string, log *zap.SugaredLogger) ([]*commonmodels.TaskArgs, error) {
	tasks := make([]*commonmodels.TaskArgs, 0)

	var (
		owner         = *event.Repo.Owner.Login
		repo          = *event.Repo.Name
		prNum         = *event.PullRequest.Number
		branch        = *event.PullRequest.Base.Ref
		commitID      = *event.PullRequest.Head.SHA
		commitMessage = *event.PullRequest.Title
	)

	address, err := util.GetAddress(event.Repo.GetHTMLURL())
	if err != nil {
		log.Errorf("GetAddress failed, url: %s, err: %s", event.Repo.GetURL(), err)
		return nil, err
	}
	ch, err := systemconfig.GetCodeHostInfo(
		&systemconfig.Option{CodeHostType: systemconfig.GitHubProvider, Address: address, Namespace: owner})
	if err != nil {
		log.Errorf("GetCodeHostInfo failed, err: %v", err)
		return nil, err
	}
	gc := githubtool.NewClient(&githubtool.Config{AccessToken: ch.AccessToken, Proxy: config.ProxyHTTPSAddr()})
	commitFiles, _ := gc.ListFiles(context.Background(), owner, repo, prNum, &githubtool.ListOptions{PerPage: 100})

	var files []string
	for _, cf := range commitFiles {
		files = append(files, *cf.Filename)
	}

	pipelineNames := listPipelinesByHook("pull_request", repo, branch, files, log)
	if len(pipelineNames) == 0 {
		log.Infof("no pipeline found by event %s %d", "pr", event.PullRequest.ID)
		return tasks, nil
	}

	eventRepo := &types.Repository{
		RepoOwner:     owner,
		RepoNamespace: owner,
		RepoName:      repo,
		Branch:        branch,
		PR:            prNum,
		CommitID:      commitID,
		CommitMessage: commitMessage,
		IsPrimary:     true,
	}

	for _, pName := range pipelineNames {

		ptargs := &commonmodels.TaskArgs{
			PipelineName: pName,
			TaskCreator:  setting.WebhookTaskCreator,
			ReqID:        requestID,
			HookPayload: &commonmodels.HookPayload{
				Owner:  owner,
				Repo:   repo,
				Ref:    commitID,
				IsPr:   true,
				Branch: branch,
			},
			Builds: []*types.Repository{eventRepo},
			Test: commonmodels.TestArgs{
				Builds: []*types.Repository{eventRepo},
			},
			CodeHostID: ch.ID,
		}
		tasks = append(tasks, ptargs)
	}

	return tasks, nil
}

// listPipelinesByHook 根据 repo,branch,event, folder来查找对应的pipelines
func listPipelinesByHook(event, repo, branch string, files []string, log *zap.SugaredLogger) []string {
	names := []string{}

	// 去掉重复的pipeline
	piplines := make(map[string]bool)
	pHookMap := buildPipelineSearchMap(log)
	key := fmt.Sprintf("%s:%s:%s", repo, branch, event)

	if _, ok := pHookMap[key]; ok {
		for _, ph := range pHookMap[key] {
			found := false
			for _, file := range files {
				if ContainsFile(&ph.GitHook, file) {
					found = true
					break
				}
			}
			if found {
				piplines[ph.PipelineName] = true
			}
		}
	}

	for key := range piplines {
		names = append(names, key)
	}

	return names
}

type PipelineHook struct {
	PipelineName string
	GitHook      commonmodels.GitHook
}

func buildPipelineSearchMap(log *zap.SugaredLogger) map[string][]PipelineHook {
	searchMap := make(map[string][]PipelineHook)

	list, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("PipelineV2.List error: %v", err)
		return searchMap
	}

	for _, p := range list {
		if p.Hook == nil || !p.Hook.Enabled {
			continue
		}

		// 查找激活的 webhook
		for _, hook := range p.Hook.GitHooks {
			for _, event := range hook.Events {
				key := fmt.Sprintf("%s:%s:%s", hook.Repo, hook.Branch, event)
				value := PipelineHook{
					PipelineName: p.Name,
					GitHook:      hook,
				}

				searchMap[key] = append(searchMap[key], value)
			}
		}
	}

	return searchMap
}

func pushEventToPipelineTasks(event *github.PushEvent, requestID string, log *zap.SugaredLogger) ([]*commonmodels.TaskArgs, error) {
	tasks := make([]*commonmodels.TaskArgs, 0)

	var (
		owner         = *event.Repo.Owner.Name
		repo          = *event.Repo.Name
		branch        = pushEventBranch(event)
		ref           = *event.Ref
		commitID      = *event.HeadCommit.ID
		commitMessage = *event.HeadCommit.Message
	)

	address, err := util.GetAddress(event.Repo.GetURL())
	if err != nil {
		log.Errorf("GetAddress failed, url: %s, err: %s", event.Repo.GetURL(), err)
		return nil, err
	}
	ch, err := systemconfig.GetCodeHostInfo(
		&systemconfig.Option{CodeHostType: systemconfig.GitHubProvider, Address: address, Namespace: owner})
	if err != nil {
		log.Errorf("GetCodeHostInfo failed, err: %v", err)
		return nil, err
	}

	pipelineNames := listPipelinesByHook("push", repo, branch, pushEventCommitsFiles(event), log)
	if len(pipelineNames) == 0 {
		log.Infof("no pipeline found by event %s %d", "push", event.PushID)
		return tasks, nil
	}

	eventRepo := &types.Repository{
		RepoOwner:     owner,
		RepoNamespace: owner,
		RepoName:      repo,
		Branch:        branch,
		CommitID:      commitID,
		CommitMessage: commitMessage,
		IsPrimary:     true,
	}

	for _, pName := range pipelineNames {
		ptargs := &commonmodels.TaskArgs{
			PipelineName: pName,
			TaskCreator:  setting.WebhookTaskCreator,
			ReqID:        requestID,
			HookPayload: &commonmodels.HookPayload{
				Owner:  owner,
				Repo:   repo,
				Branch: branch,
				Ref:    ref,
				IsPr:   false,
			},
			Builds: []*types.Repository{eventRepo},
			Test: commonmodels.TestArgs{
				Builds: []*types.Repository{eventRepo},
			},
			CodeHostID: ch.ID,
		}
		tasks = append(tasks, ptargs)
	}

	return tasks, nil
}

func pushEventBranch(e *github.PushEvent) string {
	list := strings.Split(*e.Ref, "/")
	if len(list) != 3 {
		return "unknown"
	}
	return list[2]
}

func pushEventCommitsFiles(e *github.PushEvent) []string {
	var files []string
	for _, commit := range e.Commits {
		files = append(files, commit.Added...)
		files = append(files, commit.Removed...)
		files = append(files, commit.Modified...)
	}
	return files
}

func ProcessGithubWebHookForTest(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		return nil
	}

	err := validateSecret(payload, []byte(gitservice.GetHookSecret()), req)
	if err != nil {
		return err
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return err
	}

	switch et := event.(type) {
	case *github.PullRequestEvent, *github.PushEvent, *github.CreateEvent:
		if err = TriggerTestByGithubEvent(et, requestID, log); err != nil {
			log.Errorf("TriggerTestByGithubEvent error: %s", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	default:
		log.Warn("Unsupported event type")
		return nil
	}
	return nil
}

func ProcessGithubWebhookForScanning(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		return nil
	}

	err := validateSecret(payload, []byte(gitservice.GetHookSecret()), req)
	if err != nil {
		return err
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		log.Errorf("Failed to parse webhook, err: %s", err)
		return err
	}

	deliveryID := github.DeliveryID(req)

	log.Infof("[Webhook] event: %s delivery id: %s received for scanning trigger", hookType, deliveryID)

	switch et := event.(type) {
	case *github.PullRequestEvent, *github.PushEvent, *github.CreateEvent:
		if err = TriggerScanningByGithubEvent(et, requestID, log); err != nil {
			log.Errorf("TriggerScanningByGithubEvent error: %s", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	default:
		log.Warn("Unsupported event type")
		return nil
	}
	return nil
}

func ProcessGithubWebHookForWorkflowV4(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseURI := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		return nil
	}

	err := validateSecret(payload, []byte(gitservice.GetHookSecret()), req)
	if err != nil {
		return err
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return err
	}

	deliveryID := github.DeliveryID(req)
	log.Infof("[Webhook] event: %s delivery id: %s received", hookType, deliveryID)

	switch et := event.(type) {
	case *github.PullRequestEvent:
		if *et.Action != "opened" && *et.Action != "synchronize" {
			return nil
		}
		err = TriggerWorkflowV4ByGithubEvent(et, baseURI, deliveryID, requestID, log)
		if err != nil {
			log.Errorf("prEventToPipelineTasks error: %v", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	case *github.PushEvent:
		err = TriggerWorkflowV4ByGithubEvent(et, baseURI, deliveryID, requestID, log)
		if err != nil {
			log.Infof("pushEventToPipelineTasks error: %v", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	case *github.CreateEvent:
		err = TriggerWorkflowV4ByGithubEvent(et, baseURI, deliveryID, requestID, log)
		if err != nil {
			log.Errorf("tagEventToPipelineTasks error: %s", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	}
	return nil
}

const (
	EventTypePR   = "pr"
	EventTypePush = "push"
	EventTypeTag  = "tag"
)

type AutoCancelOpt struct {
	Type           string
	MergeRequestID string
	Ref            string
	CommitID       string
	TaskType       config.PipelineType
	MainRepo       *commonmodels.MainHookRepo
	WorkflowArgs   *commonmodels.WorkflowTaskArgs
	TestArgs       *commonmodels.TestTaskArgs
	IsYaml         bool
	AutoCancel     bool
	YamlHookPath   string
	WorkflowName   string
}

func updateServiceTemplateByGithubPush(pushEvent *github.PushEvent, log *zap.SugaredLogger) error {
	changeFiles := make([]string, 0)
	for _, commit := range pushEvent.Commits {
		changeFiles = append(changeFiles, commit.Added...)
		changeFiles = append(changeFiles, commit.Removed...)
		changeFiles = append(changeFiles, commit.Modified...)
	}

	latestCommitID := *pushEvent.After
	latestCommitMessage := ""
	for _, commit := range pushEvent.Commits {
		if *commit.ID == latestCommitID {
			latestCommitMessage = *commit.Message
			break
		}
	}

	svcTmplsMap := map[bool][]*commonmodels.Service{}
	serviceTmpls, err := GetGithubTestingServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get github testing service templates, error: %v", err)
		return err
	}
	svcTmplsMap[false] = serviceTmpls
	productionServiceTmpls, err := GetGithubProductionServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get github proudction service templates, error: %v", err)
		return err
	}
	svcTmplsMap[true] = productionServiceTmpls

	errs := &multierror.Error{}
	for production, serviceTmpls := range svcTmplsMap {
		for _, service := range serviceTmpls {
			if service.GetRepoNamespace()+"/"+service.RepoName != pushEvent.GetRepo().GetFullName() {
				continue
			}

			if !checkBranchMatch(service, production, pushEvent.GetRef(), log) {
				continue
			}

			path, err := getServiceSrcPath(service)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			// 判断PushEvent的Diffs中是否包含该服务模板的src_path
			affected := false
			for _, changeFile := range changeFiles {
				if subElem(path, changeFile) {
					affected = true
					break
				}
			}
			if affected {
				log.Infof("Started to sync service template %s from github %s", service.ServiceName, service.SrcPath)
				//TODO: 异步处理
				service.CreateBy = "system"
				service.Production = production
				err := SyncServiceTemplateFromGithub(service, latestCommitID, latestCommitMessage, log)
				if err != nil {
					log.Errorf("SyncServiceTemplateFromGithub failed, error: %v", err)
					errs = multierror.Append(errs, err)
				}
			} else {
				log.Infof("Service template %s from github %s is not affected, no sync", service.ServiceName, service.SrcPath)
			}
		}
	}

	return errs.ErrorOrNil()
}

func GetGithubTestingServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGithub,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

func GetGithubProductionServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGithub,
	}
	return commonrepo.NewProductionServiceColl().ListMaxRevisions(opt)
}

// GetOwnerRepoBranchPath 获取gitlab路径中的owner、repo、branch和path
// return address, owner, repo, branch, path, pathType
// Note this function needs be optimized badly!
func GetOwnerRepoBranchPath(URL string) (string, string, string, string, string, string, error) {
	if !strings.Contains(URL, "https") && !strings.Contains(URL, "http") {
		return "", "", "", "", "", "", fmt.Errorf("url is illegal")
	}
	//适配公网的gitlab
	if strings.Contains(URL, "-") {
		URL = strings.Replace(URL, "-/", "", -1)
	}

	pathType := "tree"
	if strings.Contains(URL, "blob") {
		pathType = "blob"
	}

	urlPathArray := strings.Split(URL, "/")
	if len(urlPathArray) < 8 {
		return "", "", "", "", "", "", fmt.Errorf("url is illegal")
	}

	address, err := GetAddress(URL)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	// 如果是非根文件夹或文件
	if strings.Contains(URL, "tree") || strings.Contains(URL, "blob") {
		pathIndex := strings.Index(URL, urlPathArray[6]) + len(urlPathArray[6]) + 1
		return address, urlPathArray[3], urlPathArray[4], urlPathArray[6], URL[pathIndex:], pathType, nil
	}
	return address, urlPathArray[3], urlPathArray[4], "", "", pathType, nil
}

func GetAddress(URL string) (string, error) {
	if !strings.Contains(URL, "https") && !strings.Contains(URL, "http") {
		return "", fmt.Errorf("url is illegal")
	}
	uri, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("url prase failed")
	}
	return fmt.Sprintf("%s://%s", uri.Scheme, uri.Host), nil
}

// SyncServiceTemplateFromGithub Force to sync Service Template to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromGithub(service *commonmodels.Service, latestCommitID, latestCommitMessage string, log *zap.SugaredLogger) error {
	// 判断一下Source字段，如果Source字段不是github，直接返回
	if service.Source != setting.SourceFromGithub {
		log.Error("Service template is not from github")
		return errors.New("service template is not from github")
	}
	// 获取当前Commit的SHA
	var before string
	if service.Commit != nil {
		before = service.Commit.SHA
	}

	// 更新commit信息
	service.Commit = &commonmodels.Commit{
		SHA:     latestCommitID,
		Message: latestCommitMessage,
	}

	if before == latestCommitID {
		log.Infof("Before and after SHA: %s remains the same, no need to sync, source:%s", before, service.Source)
		// 无需更新
		return nil
	}
	// 在Ensure过程中会检查source，如果source为github，则同步github内容到service中
	if err := fillServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
		log.Errorf("ensure github serviceTmpl failed, error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}

	log.Infof("End of sync service template %s from github path %s", service.ServiceName, service.SrcPath)
	return nil
}
