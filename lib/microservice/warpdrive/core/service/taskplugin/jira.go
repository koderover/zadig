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

package taskplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/pkg/errors"
	"github.com/xanzy/go-gitlab"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	git "github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskplugin/github"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/jira"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

// InitializeJiraTaskPlugin to init plugin
func InitializeJiraTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &JiraPlugin{
		Name: taskType,
	}
}

// JiraPlugin name should be compatible with task type
type JiraPlugin struct {
	Name config.TaskType
	Task *task.Jira
	Log  *xlog.Logger
}

func (p *JiraPlugin) SetAckFunc(func()) {
}

const (
	// JiraTimeout ...
	JiraTimeout = 60 * 5 // 5 minutes
)

// Init ...
func (p *JiraPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *JiraPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *JiraPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *JiraPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *JiraPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = JiraTimeout
	}

	return p.Task.Timeout
}

func (p *JiraPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.Task.Issues = make([]*task.JiraIssue, 0, 0)

	jiraKeys := make([]string, 0, 0)

	for _, build := range p.Task.Builds {
		if build == nil {
			p.Task.TaskStatus = config.StatusSkipped
			p.Task.Error = "nil build info"
			return
		}
		if build.UseDefault == true {
			log.Warnf("[jira]skip default repo [%s]", build.RepoName)
			continue
		}
		if build.PR == 0 && build.Branch == "" {
			p.Task.TaskStatus = config.StatusSkipped
			p.Task.Error = "no pull request number and branch found"
			return
		}

		if build.Source == setting.SourceFromGitlab {
			gitlabCli := p.getGitlabClient(build.Address, build.OauthToken)
			//第一: 获取最近一次的pr
			var MergeRequest *gitlab.MergeRequest
			if build.PR == 0 {
				state := "merged"
				opt := &gitlab.ListMergeRequestsOptions{
					State:        &state,
					SourceBranch: &build.Branch,
					ListOptions:  gitlab.ListOptions{Page: 1, PerPage: 1},
				}
				mergeRequests, _, err := gitlabCli.MergeRequests.ListMergeRequests(opt)
				if err != nil {
					p.Log.Errorf("gitlab ListMergeRequests [%s/%s:%s] error: %v", build.RepoOwner, build.RepoName, build.Branch, err)
					p.Task.TaskStatus = config.StatusSkipped
					p.Task.Error = err.Error()
					return
				}
				if len(mergeRequests) > 0 {
					MergeRequest = mergeRequests[0]
				} else {
					log.Warnf("[%s/%s:%s]gitlab pull request not found", build.RepoOwner, build.RepoName, build.Branch)
				}
			} else {
				// 1. parse pr title
				mergeRequest, _, err := gitlabCli.MergeRequests.GetMergeRequest(fmt.Sprintf("%s/%s", build.RepoOwner, build.RepoName), build.PR, nil)
				if err != nil {
					p.Log.Errorf("gitlab GetMergeRequest [%s/%s:%d] error: %v", build.RepoOwner, build.RepoName, build.PR, err)
					p.Task.TaskStatus = config.StatusSkipped
					p.Task.Error = err.Error()
					return
				}
				MergeRequest = mergeRequest
			}
			if MergeRequest != nil && MergeRequest.Title != "" {
				p.Log.Infof("MergeRequest title : %s", MergeRequest.Title)
				keys := util.GetJiraKeys(MergeRequest.Title)
				if len(keys) > 0 {
					jiraKeys = append(jiraKeys, keys...)
				}
			}
			// 2. parse all commits
			repoCommits := make([]*gitlab.Commit, 0)
			if build.PR != 0 {
				opt := &gitlab.GetMergeRequestCommitsOptions{Page: 1, PerPage: 100}
				for opt.Page > 0 {
					list, resp, err := gitlabCli.MergeRequests.GetMergeRequestCommits(fmt.Sprintf("%s/%s", build.RepoOwner, build.RepoName), build.PR, opt)
					if err != nil {
						p.Log.Errorf("gitlab GetMergeRequestCommits [%s/%s:%d] error: %v", build.RepoOwner, build.RepoName, build.PR, err)
						p.Task.TaskStatus = config.StatusSkipped
						p.Task.Error = err.Error()
						return
					}
					repoCommits = append(repoCommits, list...)
					opt.Page = resp.NextPage
				}
				for _, rc := range repoCommits {
					// parse commit message
					if rc != nil && rc.Message != "" {
						keys := util.GetJiraKeys(rc.Message)
						if len(keys) > 0 {
							jiraKeys = append(jiraKeys, keys...)
						}
					}
				}
			}
		} else {
			httpsAddr := ""
			if pipelineTask.ConfigPayload.Proxy.EnableRepoProxy && pipelineTask.ConfigPayload.Proxy.Type == "http" {
				httpsAddr = pipelineTask.ConfigPayload.Proxy.GetProxyUrl()
			}
			githubCli := git.NewGithubAppClient(build.OauthToken, config.APIServer, httpsAddr)
			//第一: 获取最近一次的pr
			var pr *github.PullRequest
			if build.PR == 0 {
				opt := &github.PullRequestListOptions{
					State:       "closed",
					Base:        build.Branch,
					ListOptions: github.ListOptions{Page: 1, PerPage: 1},
				}
				pullrequest, _, err := githubCli.PullRequests.List(context.Background(), build.RepoOwner, build.RepoName, opt)
				if err != nil {
					p.Log.Errorf("github ListClosedPullRequests [%s/%s:%s] error: %v", build.RepoOwner, build.RepoName, build.Branch, err)
					p.Task.TaskStatus = config.StatusSkipped
					p.Task.Error = err.Error()
					return
				}
				if len(pullrequest) > 0 {
					pr = pullrequest[0]
				} else {
					log.Warnf("[%s/%s:%s]github pull request not found,", build.RepoOwner, build.RepoName, build.Branch)
				}
			} else {
				// 1. parse pr title
				pullrequest, _, err := githubCli.PullRequests.Get(context.Background(), build.RepoOwner, build.RepoName, build.PR)
				if err != nil {
					p.Log.Errorf("github GetPullRequest [%s/%s:%d] error: %v", build.RepoOwner, build.RepoName, build.PR, err)
					p.Task.TaskStatus = config.StatusSkipped
					p.Task.Error = err.Error()
					return
				}
				pr = pullrequest
			}
			if pr != nil && pr.Title != nil {
				p.Log.Infof("pullRequest title : %s", *pr.Title)
				keys := util.GetJiraKeys(*pr.Title)
				if len(keys) > 0 {
					jiraKeys = append(jiraKeys, keys...)
				}
			}

			// 2. parse all commits
			repoCommits := make([]*github.RepositoryCommit, 0)
			if build.PR != 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				for opt.Page > 0 {
					list, resp, err := githubCli.PullRequests.ListCommits(context.Background(), build.RepoOwner, build.RepoName, build.PR, opt)
					if err != nil {
						p.Log.Errorf("github ListRepoCommitsByPullRequest [%s/%s:%d] error: %v", build.RepoOwner, build.RepoName, build.PR, err)
						p.Task.TaskStatus = config.StatusSkipped
						p.Task.Error = err.Error()
						return
					}
					repoCommits = append(repoCommits, list...)
					opt.Page = resp.NextPage
				}

				for _, rc := range repoCommits {
					// parse commit message
					if rc.Commit != nil && rc.Commit.Message != nil {
						keys := util.GetJiraKeys(*rc.Commit.Message)
						if len(keys) > 0 {
							jiraKeys = append(jiraKeys, keys...)
						}
					}
				}
			}

		}
	}

	jiraKeys = removeDuplicateKey(jiraKeys)
	p.Log.Infof("jiraKeys : %v", jiraKeys)

	for _, key := range jiraKeys {
		issue, err := p.getJiraIssue(pipelineTask, key)
		if err != nil {
			p.Log.Errorf("get jira issue [%s] error: %v", key, err)
			continue
		}
		p.Task.Issues = append(p.Task.Issues, issue)
	}

	p.Task.TaskStatus = config.StatusPassed
	return
}

// Wait ...
func (p *JiraPlugin) Wait(ctx context.Context) {
	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return

		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout
			return

		default:
			time.Sleep(time.Second * 1)

			if p.IsTaskDone() {
				return
			}
		}
	}
}

func (p *JiraPlugin) getGitlabClient(host, token string) *gitlab.Client {
	cli, _ := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(host))
	return cli
}

// Complete ...
func (p *JiraPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

// SetTask ...
func (p *JiraPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToJiraTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *JiraPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *JiraPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *JiraPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *JiraPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *JiraPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// -------------------------------------------------------------------------------
// helper functions
// -------------------------------------------------------------------------------
func (p *JiraPlugin) getJiraIssue(pipelineTask *task.Task, key string) (*task.JiraIssue, error) {
	jiraIssue := new(task.JiraIssue)
	jiraInfo, err := GetJiraInfo(config.PoetryAPIAddr(), config.PoetryAPIRootKey())
	if err != nil {
		return nil, fmt.Errorf("getJiraInfo [%s] error: %v", key, err)
	}
	jiraCli := jira.NewJiraClient(jiraInfo.User, jiraInfo.AccessToken, jiraInfo.Host)
	issue, err := jiraCli.Issue.GetByKeyOrID(key, "")
	if err != nil {
		return jiraIssue, fmt.Errorf("GetIssueByKeyOrID [%s] error: %v", key, err)
	}

	jiraIssue.ID = issue.ID
	jiraIssue.Key = issue.Key
	jiraIssue.URL = strings.Join([]string{jiraInfo.Host, "browse", issue.Key}, "/")

	if issue.Fields != nil {
		jiraIssue.Summary = issue.Fields.Summary
		jiraIssue.Description = issue.Fields.Description

		if issue.Fields.Assignee != nil {
			jiraIssue.Assignee = issue.Fields.Assignee.Name
		}
		if issue.Fields.Creator != nil {
			jiraIssue.Creator = issue.Fields.Creator.Name
		}
		if issue.Fields.Reporter != nil {
			jiraIssue.Reporter = issue.Fields.Reporter.Name
		}
		if issue.Fields.Priority != nil {
			jiraIssue.Priority = issue.Fields.Priority.Name
		}
	}
	return jiraIssue, nil
}

// IsTaskEnabled ...
func (p *JiraPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *JiraPlugin) ResetError() {
	p.Task.Error = ""
}

func removeDuplicateKey(slc []string) []string {
	dic := make(map[string]bool)
	result := []string{} // 存放结果
	for _, key := range slc {
		if !dic[key] {
			dic[key] = true
			result = append(result, key)
		}
	}
	return result
}

type JiraInfo struct {
	ID             int64  `json:"id"`
	Host           string `json:"host"`
	User           string `json:"user"`
	AccessToken    string `json:"accessToken"`
	OrganizationID int    `json:"organizationId"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func GetJiraInfo(poetryApiServer, ApiRootKey string) (*JiraInfo, error) {

	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, ApiRootKey))
	header.Set("content-type", "application/json")

	url := poetryApiServer + "/directory/jira?orgId=1"
	data, err := util.SendRequest(url, http.MethodGet, header, nil)
	if err != nil {
		log.Error("SendRequest err :", err)
		return nil, errors.WithStack(err)
	}

	var Jira *JiraInfo
	if err := json.Unmarshal(data, &Jira); err != nil {
		log.Error("GetJiraInfo Unmarshal err :", err)
		return nil, errors.WithMessage(err, string(data))
	}

	return Jira, nil
}
