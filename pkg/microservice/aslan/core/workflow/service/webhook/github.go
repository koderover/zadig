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
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collie"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/pkg/tool/git/github"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

const (
	signatureHeader  = "X-Hub-Signature"
	payloadFormParam = "payload"
)

func ProcessGithubHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) (output string, err error) {
	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		output = fmt.Sprintf("event %s received", hookType)
		return
	}

	hookSecret := gitservice.GetHookSecret()

	if hookSecret == "" {
		var headers []string
		for header := range req.Header {
			headers = append(headers, fmt.Sprintf("%s: %s", header, req.Header.Get(header)))
		}

		log.Infof("[Webhook] hook headers: \n  %s", strings.Join(headers, "\n  "))
	}

	err = validateSecret(payload, []byte(hookSecret), req)
	if err != nil {
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return
	}

	deliveryID := github.DeliveryID(req)

	log.Infof("[Webhook] event: %s delivery id: %s received", hookType, deliveryID)

	var tasks []*commonmodels.TaskArgs
	switch et := event.(type) {
	case *github.PullRequestEvent:
		if *et.Action != "opened" && *et.Action != "synchronize" && *et.Action != "reopened" {
			output = fmt.Sprintf("action %s is skipped", *et.Action)
			return
		}

		tasks, err = prEventToPipelineTasks(et, requestID, log)
		if err != nil {
			log.Errorf("prEventToPipelineTasks error: %v", err)
			err = e.ErrGithubWebHook.AddErr(err)
			return
		}

	case *github.PushEvent:
		//add webhook user
		if et.Pusher != nil {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  *et.Pusher.Name,
				Email:     *et.Pusher.Email,
				Source:    setting.SourceFromGithub,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}

		tasks, err = pushEventToPipelineTasks(et, requestID, log)
		if err != nil {
			log.Infof("pushEventToPipelineTasks error: %v", err)
			err = e.ErrGithubWebHook.AddErr(err)
			return
		}
	case *github.CheckRunEvent:
		// The action performed. Can be "created", "updated", "rerequested" or "requested_action".
		if *et.Action != "rerequested" {
			output = fmt.Sprintf("action %s is skipped", *et.Action)
			return
		}

		id := et.CheckRun.GetExternalID()
		items := strings.Split(id, "/")
		if len(items) != 2 {
			err = fmt.Errorf("invalid CheckRun ExternalID %s", id)
			return
		}

		pipeName := items[0]
		var taskID int64
		taskID, err = strconv.ParseInt(items[1], 10, 64)
		if err != nil {
			err = fmt.Errorf("invalid taskID in CheckRun ExternalID %s", id)
			return
		}

		if err = workflowservice.RestartPipelineTaskV2("CheckRun", taskID, pipeName, config.SingleType, log); err != nil {
			err = e.ErrGithubWebHook.AddErr(err)
			return
		}

	default:
		output = fmt.Sprintf("event %s not support", hookType)
		return
	}

	for _, task := range tasks {
		task.HookPayload.DeliveryID = deliveryID
		// 暂时不 block webhook 请求
		resp, err1 := workflowservice.CreatePipelineTask(task, log)
		if err1 != nil {
			log.Errorf("[Webhook] %s triggered task %s error: %v", deliveryID, task.PipelineName, err)
			continue
		}
		log.Infof("[Webhook] %s triggered task %s:%d", deliveryID, task.PipelineName, resp.TaskID)
	}

	return
}

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

	address, err := util.GetAddress(event.Repo.GetURL())
	if err != nil {
		log.Errorf("GetAddress failed, url: %s, err: %s", event.Repo.GetURL(), err)
		return nil, err
	}
	ch, err := codehost.GetCodeHostInfo(
		&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
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
	ch, err := codehost.GetCodeHostInfo(
		&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
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

func ProcessGithubWebHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
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

	go collie.CallGithubWebHook(forwardedProto, forwardedHost, payload, github.WebHookType(req), log)

	deliveryID := github.DeliveryID(req)
	log.Infof("[Webhook] event: %s delivery id: %s received", hookType, deliveryID)

	switch et := event.(type) {
	case *github.PullRequestEvent:
		if *et.Action != "opened" && *et.Action != "synchronize" {
			return nil
		}

		err = TriggerWorkflowByGithubEvent(et, baseURI, deliveryID, requestID, log)
		if err != nil {
			log.Errorf("prEventToPipelineTasks error: %v", err)
			return e.ErrGithubWebHook.AddErr(err)
		}

	case *github.PushEvent:
		//触发更新服务模板webhook
		if pushEvent, isPush := event.(*github.PushEvent); isPush {
			if err = updateServiceTemplateByGithubPush(pushEvent, log); err != nil {
				log.Errorf("updateServiceTemplateByGithubPush failed, error:%v", err)
			}
		}

		//add webhook user
		if et.Pusher != nil {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  *et.Pusher.Name,
				Email:     *et.Pusher.Email,
				Source:    setting.SourceFromGithub,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}
		err = TriggerWorkflowByGithubEvent(et, baseURI, deliveryID, requestID, log)
		if err != nil {
			log.Infof("pushEventToPipelineTasks error: %v", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	}
	return nil
}

type AutoCancelOpt struct {
	MergeRequestID string
	CommitID       string
	TaskType       config.PipelineType
	MainRepo       commonmodels.MainHookRepo
	WorkflowArgs   *commonmodels.WorkflowTaskArgs
	TestArgs       *commonmodels.TestTaskArgs
}

func getProductTargetMap(prod *commonmodels.Product) map[string][]commonmodels.DeployEnv {
	resp := make(map[string][]commonmodels.DeployEnv)
	for _, services := range prod.Services {
		for _, serviceObj := range services {
			switch serviceObj.Type {
			case setting.K8SDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.K8SDeployType, Env: env}

					target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, container.Name)

					resp[target] = append(resp[target], deployEnv)
				}
			case setting.PMDeployType:
				deployEnv := commonmodels.DeployEnv{Type: setting.PMDeployType, Env: serviceObj.ServiceName}
				target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, serviceObj.ServiceName)
				resp[target] = append(resp[target], deployEnv)
			case setting.HelmDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.HelmDeployType, Env: env}

					target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, container.Name)
					resp[target] = append(resp[target], deployEnv)
				}
			}
		}
	}
	return resp
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

	serviceTmpls, err := GetGithubServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get github service templates, error: %v", err)
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
		for _, changeFile := range changeFiles {
			if strings.Contains(changeFile, path) {
				affected = true
				break
			}
		}
		if affected {
			log.Infof("Started to sync service template %s from github %s", service.ServiceName, service.SrcPath)
			//TODO: 异步处理
			service.CreateBy = "system"
			err := SyncServiceTemplateFromGithub(service, latestCommitID, latestCommitMessage, log)
			if err != nil {
				log.Errorf("SyncServiceTemplateFromGithub failed, error: %v", err)
				errs = multierror.Append(errs, err)
			}
		} else {
			log.Infof("Service template %s from github %s is not affected, no sync", service.ServiceName, service.SrcPath)
		}
	}
	return errs.ErrorOrNil()
}

func GetGithubServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceFindOption{
		Type:          setting.K8SDeployType,
		Source:        setting.SourceFromGithub,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	return commonrepo.NewServiceColl().List(opt)
}

// GetOwnerRepoBranchPath 获取gitlab路径中的owner、repo、branch和path
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

	address, err := GetGitlabAddress(URL)
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

func GetGitlabAddress(URL string) (string, error) {
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
	// 更新到数据库，revision+1
	if err := commonrepo.NewServiceColl().Create(service); err != nil {
		log.Errorf("Failed to sync service %s from github path %s error: %v", service.ServiceName, service.SrcPath, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s from github path %s", service.ServiceName, service.SrcPath)
	return nil
}
