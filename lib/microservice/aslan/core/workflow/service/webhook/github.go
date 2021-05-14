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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github/app"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
)

const (
	signatureHeader  = "X-Hub-Signature"
	payloadFormParam = "payload"
	githubAPIServer  = "https://api.github.com/"
)

func ProcessGithubHook(req *http.Request, log *xlog.Logger) (output string, err error) {
	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		output = fmt.Sprintf("event %s received", hookType)
		return
	}

	hookSecret := getHookSecret(log)

	if hookSecret == "" {
		var headers []string
		for header := range req.Header {
			headers = append(headers, fmt.Sprintf("%s: %s", header, req.Header.Get(header)))
		}

		log.Infof("[Webhook] hook headers: \n  %s", strings.Join(headers, "\n  "))
	}

	var payload []byte
	payload, err = ValidatePayload(req, []byte(hookSecret), log)
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

		tasks, err = prEventToPipelineTasks(et, log)
		if err != nil {
			log.Infof("prEventToPipelineTasks error: %v", err)
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

		tasks, err = pushEventToPipelineTasks(et, log)
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

var hookSecret string
var hookSecretSet bool

func getHookSecret(logger *xlog.Logger) string {
	if !hookSecretSet {
		switch config.HookSecret() {
		case "ORG_TOKEN":
			poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
			org, err := poetryClient.GetOrg(1)
			if err != nil {
				logger.Errorf("failed to find default organization: %v", err)
				return "--impossible-token--"
			}

			hookSecret = org.Token
		default:
			hookSecret = config.HookSecret()
		}
		hookSecretSet = true
	}

	return hookSecret
}

func ValidatePayload(r *http.Request, secretKey []byte, log *xlog.Logger) (payload []byte, err error) {
	var body []byte // Raw body that GitHub uses to calculate the signature.

	switch ct := r.Header.Get("Content-Type"); ct {
	case "application/json":
		var err error
		if body, err = ioutil.ReadAll(r.Body); err != nil {
			return nil, err
		}

		// If the content type is application/json,
		// the JSON payload is just the original body.
		payload = body

	case "application/x-www-form-urlencoded":
		// payloadFormParam is the name of the form parameter that the JSON payload
		// will be in if a webhook has its content type set to application/x-www-form-urlencoded.

		var err error
		if body, err = ioutil.ReadAll(r.Body); err != nil {
			return nil, err
		}

		// If the content type is application/x-www-form-urlencoded,
		// the JSON payload will be under the "payload" form param.
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		payload = []byte(form.Get(payloadFormParam))

	default:
		return nil, fmt.Errorf("webhook request has unsupported Content-Type %q", ct)
	}

	sig := r.Header.Get(signatureHeader)
	if len(secretKey) > 0 {
		if err := github.ValidateSignature(sig, body, secretKey); err != nil {
			return nil, err
		}
	} else {
		log.Infof("[Webhook] got payload %s", string(payload))
	}
	return payload, nil
}

func prEventToPipelineTasks(event *github.PullRequestEvent, log *xlog.Logger) ([]*commonmodels.TaskArgs, error) {
	tasks := make([]*commonmodels.TaskArgs, 0)

	var (
		owner         = *event.Repo.Owner.Login
		repo          = *event.Repo.Name
		prNum         = *event.PullRequest.Number
		branch        = *event.PullRequest.Base.Ref
		commitID      = *event.PullRequest.Head.SHA
		commitMessage = *event.PullRequest.Title
	)

	gitCli, err := initGithubClient(owner)
	if err != nil {
		return tasks, err
	}

	commitFiles := make([]*github.CommitFile, 0)

	opt := &github.ListOptions{Page: 1, PerPage: 100}
	for opt.Page > 0 {
		files, resp, err := gitCli.Git.PullRequests.ListFiles(context.Background(), owner, repo, prNum, opt)
		if err != nil {
			return tasks, err
		}
		commitFiles = append(commitFiles, files...)
		opt.Page = resp.NextPage
	}

	files := []string{}
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
			ReqID:        log.ReqID(),
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
		}
		tasks = append(tasks, ptargs)
	}

	return tasks, nil
}

func initGithubClient(owner string) (*git.Client, error) {
	githubApps, err := commonrepo.NewGithubAppColl().Find()
	if len(githubApps) == 0 {
		return nil, err
	}

	appKey := githubApps[0].AppKey
	appID := githubApps[0].AppID

	client, err := getGithubAppCli(appKey, appID)
	if client == nil {
		return nil, err
	}
	installID, err := client.FindInstallationID(owner)
	if err != nil {
		return nil, err
	}

	gitCfg := &git.Config{
		AppKey:         appKey,
		AppID:          appID,
		InstallationID: installID,
		ProxyAddr:      config.ProxyHTTPSAddr(),
	}

	appCli, err := git.NewDynamicClient(gitCfg)
	if err != nil {
		return nil, err
	}

	return appCli, nil
}

func getGithubAppCli(appKey string, appID int) (*app.Client, error) {
	return app.NewAppClient(&app.Config{
		AppKey: appKey,
		AppID:  appID,
	})
}

// listPipelinesByHook 根据 repo,branch,event, folder来查找对应的pipelines
func listPipelinesByHook(event, repo, branch string, files []string, log *xlog.Logger) []string {
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

func buildPipelineSearchMap(log *xlog.Logger) map[string][]PipelineHook {
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
		if p.Hook != nil && p.Hook.Enabled {
			for _, hook := range p.Hook.GitHooks {
				for _, event := range hook.Events {
					key := fmt.Sprintf("%s:%s:%s", hook.Repo, hook.Branch, event)
					value := PipelineHook{
						PipelineName: p.Name,
						GitHook:      hook,
					}
					if _, ok := searchMap[key]; !ok {
						searchMap[key] = []PipelineHook{value}
					} else {
						searchMap[key] = append(searchMap[key], value)
					}
				}
			}
		}
	}

	return searchMap
}

func pushEventToPipelineTasks(event *github.PushEvent, log *xlog.Logger) ([]*commonmodels.TaskArgs, error) {
	tasks := make([]*commonmodels.TaskArgs, 0)

	var (
		owner         = *event.Repo.Owner.Name
		repo          = *event.Repo.Name
		branch        = pushEventBranch(event)
		ref           = *event.Ref
		commitID      = *event.HeadCommit.ID
		commitMessage = *event.HeadCommit.Message
	)

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
			ReqID:        log.ReqID(),
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

func ProcessGithubWebHook(req *http.Request, log *xlog.Logger) error {
	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseUri := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	hookType := github.WebHookType(req)
	if hookType == "integration_installation" || hookType == "installation" || hookType == "ping" {
		return nil
	}

	payload, err := ValidatePayload(req, []byte(getHookSecret(log)), log)
	if err != nil {
		return err
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		return err
	}

	go CallGithubWebHook(forwardedProto, forwardedHost, payload, github.WebHookType(req), log)

	deliveryID := github.DeliveryID(req)
	log.Infof("[Webhook] event: %s delivery id: %s received", hookType, deliveryID)

	switch et := event.(type) {
	case *github.PullRequestEvent:
		if *et.Action != "opened" && *et.Action != "synchronize" {
			return nil
		}

		err = TriggerWorkflowByGithubEvent(et, baseUri, deliveryID, log)
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
		err = TriggerWorkflowByGithubEvent(et, baseUri, deliveryID, log)
		if err != nil {
			log.Infof("pushEventToPipelineTasks error: %v", err)
			return e.ErrGithubWebHook.AddErr(err)
		}
	}
	return nil
}

type GitWebHookObj struct {
	Payload     string           `json:"payload"`
	EventType   gitlab.EventType `json:"event_type,omitempty"`
	MessageType string           `json:"message_type,omitempty"`
}

func CallGithubWebHook(forwardedProto, forwardedHost string, payload []byte, messageType string, log *xlog.Logger) error {
	collieApiAddress := config.CollieAPIAddress()
	if collieApiAddress == "" {
		return nil
	}
	client := poetry.NewPoetryServer(collieApiAddress, config.PoetryAPIRootKey())
	body, _ := json.Marshal(&GitWebHookObj{Payload: string(payload), MessageType: messageType})

	header := http.Header{}
	header.Add("X-Forwarded-Proto", forwardedProto)
	header.Add("X-Forwarded-Host", forwardedHost)

	_, err := client.Do("/api/collie/api/hook/github", "POST", bytes.NewBuffer(body), header)
	if err != nil {
		log.Errorf("call collie github webhook err:%+v", err)
		return err
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

func updateServiceTemplateByGithubPush(pushEvent *github.PushEvent, log *xlog.Logger) error {
	changeFiles := make([]string, 0)
	for _, commit := range pushEvent.Commits {
		changeFiles = append(changeFiles, commit.Added...)
		changeFiles = append(changeFiles, commit.Removed...)
		changeFiles = append(changeFiles, commit.Modified...)
	}

	latestCommitId := *pushEvent.After
	latestCommitMessage := ""
	for _, commit := range pushEvent.Commits {
		if *commit.ID == latestCommitId {
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
			err := SyncServiceTemplateFromGithub(service, latestCommitId, latestCommitMessage, log)
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
		return "", fmt.Errorf("url prase failed!")
	}
	return fmt.Sprintf("%s://%s", uri.Scheme, uri.Host), nil
}

// SyncServiceTemplateFromGithub Force to sync Service Template to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromGithub(service *commonmodels.Service, latestCommitId, latestCommitMessage string, log *xlog.Logger) error {
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
		SHA:     latestCommitId,
		Message: latestCommitMessage,
	}

	if before == latestCommitId {
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
