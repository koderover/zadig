/*
Copyright 2025 The KodeRover Authors.

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

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	git "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func FillRepositoryInfo(repo *types.Repository) error {
	codeHostInfo, err := systemconfig.New().GetCodeHost(repo.CodehostID)
	if err != nil {
		log.Errorf("failed to get codehost detail %d %v", repo.CodehostID, err)
		return fmt.Errorf("failed to get codehost detail %d %v", repo.CodehostID, err)
	}
	if repo.PR > 0 && len(repo.PRs) == 0 {
		repo.PRs = []int{repo.PR}
	}
	repo.Address = codeHostInfo.Address
	if codeHostInfo.Type == systemconfig.GitLabProvider || codeHostInfo.Type == systemconfig.GerritProvider {
		if repo.CommitID == "" {
			var commit *RepoCommit
			var pr *PRCommit
			var err error
			if repo.Tag != "" {
				commit, err = QueryByTag(repo.CodehostID, repo.GetRepoNamespace(), repo.RepoName, repo.Tag)
			} else if repo.Branch != "" && len(repo.PRs) == 0 {
				commit, err = QueryByBranch(repo.CodehostID, repo.GetRepoNamespace(), repo.RepoName, repo.Branch)
			} else if len(repo.PRs) > 0 {
				pr, err = GetLatestPrCommit(repo.CodehostID, getLatestPrNum(repo), repo.GetRepoNamespace(), repo.RepoName)
				if err == nil && pr != nil {
					commit = &RepoCommit{
						ID:         pr.ID,
						Message:    pr.Title,
						AuthorName: pr.AuthorName,
					}
					repo.CheckoutRef = pr.CheckoutRef
				} else {
					log.Warnf("pr setBuildInfo failed, use build:%v err:%v", repo, err)
					return nil
				}
			} else {
				return nil
			}

			if err != nil {
				log.Warnf("setBuildInfo failed, use build %+v %s", repo, err)
				return nil
			}

			repo.CommitID = commit.ID
			repo.CommitMessage = commit.Message
			repo.AuthorName = commit.AuthorName
		}
		// get gerrit submission_id
		if codeHostInfo.Type == systemconfig.GerritProvider {
			repo.SubmissionID, err = getSubmissionID(repo.CodehostID, repo.CommitID)
			log.Infof("get gerrit submissionID %s by commitID %s", repo.SubmissionID, repo.CommitID)
			if err != nil {
				log.Errorf("getSubmissionID failed, use build %+v %s", repo, err)
			}
		}
	} else if codeHostInfo.Type == systemconfig.GiteeProvider || codeHostInfo.Type == systemconfig.GiteeEEProvider {
		gitCli := gitee.NewClient(codeHostInfo.ID, codeHostInfo.Address, codeHostInfo.AccessToken, config.ProxyHTTPSAddr(), codeHostInfo.EnableProxy)
		if repo.CommitID == "" {
			if repo.Tag != "" && len(repo.PRs) == 0 {
				tags, err := gitCli.ListTags(context.Background(), codeHostInfo.Address, codeHostInfo.AccessToken, repo.RepoOwner, repo.RepoName)
				if err != nil {
					log.Errorf("failed to gitee ListTags err:%s", err)
					return fmt.Errorf("failed to gitee ListTags err:%s", err)
				}

				for _, tag := range tags {
					if tag.Name == repo.Tag {
						repo.CommitID = tag.Commit.Sha
						commitInfo, err := gitCli.GetSingleCommitOfProject(context.Background(), codeHostInfo.Address, codeHostInfo.AccessToken, repo.RepoOwner, repo.RepoName, repo.CommitID)
						if err != nil {
							log.Errorf("failed to gitee GetCommit %s err:%s", tag.Commit.Sha, err)
							return fmt.Errorf("failed to gitee GetCommit %s err:%s", tag.Commit.Sha, err)
						}
						repo.CommitMessage = commitInfo.Commit.Message
						repo.AuthorName = commitInfo.Commit.Author.Name
						return nil
					}
				}
			} else if repo.Branch != "" && len(repo.PRs) == 0 {
				branch, err := gitCli.GetSingleBranch(codeHostInfo.Address, codeHostInfo.AccessToken, repo.RepoOwner, repo.RepoName, repo.Branch)
				if err != nil {
					log.Errorf("failed to gitee GetSingleBranch  repoOwner:%s,repoName:%s,repoBranch:%s err:%s", repo.RepoOwner, repo.RepoName, repo.Branch, err)
					return fmt.Errorf("failed to gitee GetSingleBranch  repoOwner:%s,repoName:%s,repoBranch:%s err:%s", repo.RepoOwner, repo.RepoName, repo.Branch, err)
				}
				repo.CommitID = branch.Commit.Sha
				repo.CommitMessage = branch.Commit.Commit.Message
				repo.AuthorName = branch.Commit.Commit.Author.Name
			} else if len(repo.PRs) > 0 {
				prCommits, err := gitCli.ListCommitsForPR(context.Background(), repo.RepoOwner, repo.RepoName, getLatestPrNum(repo), nil)
				sort.SliceStable(prCommits, func(i, j int) bool {
					return prCommits[i].Commit.Committer.Date.Unix() > prCommits[j].Commit.Committer.Date.Unix()
				})
				if err == nil && len(prCommits) > 0 {
					for _, commit := range prCommits {
						repo.CommitID = commit.Sha
						repo.CommitMessage = commit.Commit.Message
						repo.AuthorName = commit.Commit.Author.Name
						return nil
					}
				}
			}
		}
	} else if codeHostInfo.Type == systemconfig.GitHubProvider {
		gitCli := git.NewClient(codeHostInfo.AccessToken, config.ProxyHTTPSAddr(), codeHostInfo.EnableProxy)
		if repo.CommitID == "" {
			if repo.Tag != "" && len(repo.PRs) == 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				tags, _, err := gitCli.Repositories.ListTags(context.Background(), repo.RepoOwner, repo.RepoName, opt)
				if err != nil {
					log.Errorf("failed to github ListTags err:%s", err)
					return fmt.Errorf("failed to github ListTags err:%s", err)
				}
				for _, tag := range tags {
					if *tag.Name == repo.Tag {
						repo.CommitID = tag.Commit.GetSHA()
						if tag.Commit.Message == nil {
							commitInfo, _, err := gitCli.Repositories.GetCommit(context.Background(), repo.RepoOwner, repo.RepoName, repo.CommitID)
							if err != nil {
								log.Errorf("failed to github GetCommit %s err:%s", tag.Commit.GetURL(), err)
								return fmt.Errorf("failed to github GetCommit %s err:%s", tag.Commit.GetURL(), err)
							}
							repo.CommitMessage = commitInfo.GetCommit().GetMessage()
						}
						repo.AuthorName = tag.Commit.GetAuthor().GetName()
						return nil
					}
				}
			} else if repo.Branch != "" && len(repo.PRs) == 0 {
				branch, _, err := gitCli.Repositories.GetBranch(context.Background(), repo.RepoOwner, repo.RepoName, repo.Branch)
				if err == nil {
					repo.CommitID = *branch.Commit.SHA
					repo.CommitMessage = *branch.Commit.Commit.Message
					repo.AuthorName = *branch.Commit.Commit.Author.Name
				}
			} else if len(repo.PRs) > 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				prCommits, _, err := gitCli.PullRequests.ListCommits(context.Background(), repo.RepoOwner, repo.RepoName, getLatestPrNum(repo), opt)
				sort.SliceStable(prCommits, func(i, j int) bool {
					return prCommits[i].Commit.Committer.Date.Unix() > prCommits[j].Commit.Committer.Date.Unix()
				})
				if err == nil && len(prCommits) > 0 {
					for _, commit := range prCommits {
						repo.CommitID = *commit.SHA
						repo.CommitMessage = *commit.Commit.Message
						repo.AuthorName = *commit.Commit.Author.Name
						return nil
					}
				}
			} else {
				log.Warnf("github setBuildInfo failed, use build %+v", repo)
				return nil
			}
		}
	} else if codeHostInfo.Type == systemconfig.OtherProvider {
		repo.SSHKey = codeHostInfo.SSHKey
		repo.PrivateAccessToken = codeHostInfo.PrivateAccessToken
		return nil
	}
	return nil
}

// RepoCommit : Repository commit struct
type RepoCommit struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	AuthorName string     `json:"author_name"`
	CreatedAt  *time.Time `json:"created_at"`
	Message    string     `json:"message"`
}

func QueryByBranch(id int, owner, name, branch string) (*RepoCommit, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, err
	}

	if ch.Type == setting.SourceFromGitlab {
		token, address := ch.AccessToken, ch.Address
		var client *http.Client
		if ch.EnableProxy {
			proxyURL, err := url.Parse(config.ProxyHTTPSAddr())
			if err != nil {
				return nil, err
			}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
			client = &http.Client{Transport: transport}
		} else {
			client = http.DefaultClient
		}
		cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address), gitlab.WithHTTPClient(client))
		if err != nil {
			return nil, fmt.Errorf("set base url failed, err:%v", err)
		}

		br, _, err := cli.Branches.GetBranch(owner+"/"+name, branch)
		if err != nil {
			return nil, err
		}

		return &RepoCommit{
			ID:         br.Commit.ID,
			Title:      br.Commit.Title,
			AuthorName: br.Commit.AuthorName,
			CreatedAt:  br.Commit.CreatedAt,
			Message:    br.Commit.Message,
		}, nil
	} else if ch.Type == setting.SourceFromGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		commit, err := cli.GetCommitByBranch(name, branch)
		if err != nil {
			return nil, err
		}

		return &RepoCommit{
			ID:         commit.Commit,
			Title:      commit.Subject,
			AuthorName: commit.Author.Name,
			CreatedAt:  &commit.Author.Date.Time,
			Message:    commit.Message,
		}, nil
	}

	return nil, fmt.Errorf("%s is not supported yet", ch.Type)
}

func QueryByTag(id int, owner, name, tag string) (*RepoCommit, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, err
	}

	if ch.Type == setting.SourceFromGitlab {
		token, address := ch.AccessToken, ch.Address

		cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address))
		if err != nil {
			return nil, fmt.Errorf("set base url failed, err:%v", err)
		}

		br, _, err := cli.Tags.GetTag(owner+"/"+name, tag)
		if err != nil {
			return nil, err
		}

		return &RepoCommit{
			ID:         br.Commit.ID,
			Title:      br.Commit.Title,
			AuthorName: br.Commit.AuthorName,
			CreatedAt:  br.Commit.CreatedAt,
			Message:    br.Commit.Message,
		}, nil
	} else if ch.Type == setting.SourceFromGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		commit, err := cli.GetCommitByTag(name, tag)
		if err != nil {
			return nil, err
		}

		return &RepoCommit{
			ID:         commit.Commit,
			Title:      commit.Subject,
			AuthorName: commit.Author.Name,
			CreatedAt:  &commit.Author.Date.Time,
			Message:    commit.Message,
		}, nil
	}

	return nil, fmt.Errorf("%s is not supported yet", ch.Type)
}

type PRCommit struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	AuthorName  string     `json:"author_name"`
	CreatedAt   *time.Time `json:"created_at"`
	CheckoutRef string     `json:"checkout_ref"`
}

func GetLatestPrCommit(codehostID, pr int, namespace, projectName string) (*PRCommit, error) {
	projectID := fmt.Sprintf("%s/%s", namespace, projectName)

	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, err
	}

	token, address := ch.AccessToken, ch.Address
	cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address))
	if err != nil {
		return nil, fmt.Errorf("set base url failed, err:%v", err)
	}

	if ch.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		change, err := cli.GetCurrentVersionByChangeID(projectName, pr)
		if err != nil {
			return nil, err
		}

		if _, ok := change.Revisions[change.CurrentRevision]; !ok {
			return nil, fmt.Errorf(
				"current version %s is not in revision map %v", change.CurrentRevision, change.Revisions)
		}

		tm := change.Revisions[change.CurrentRevision].Created.Time
		return &PRCommit{
			ID:          change.CurrentRevision,
			Title:       change.Subject,
			AuthorName:  change.Revisions[change.CurrentRevision].Uploader.Name,
			CreatedAt:   &tm,
			CheckoutRef: change.Revisions[change.CurrentRevision].Ref,
		}, nil
	}
	return getLatestPRCommitList(cli, projectID, pr)
}

func getLatestPRCommitList(cli *gitlab.Client, projectID string, pr int) (*PRCommit, error) {
	opts := &gitlab.GetMergeRequestCommitsOptions{
		Page:    1,
		PerPage: 10,
	}

	respMRs := make([]*PRCommit, 0)

	prCommits, _, err := cli.MergeRequests.GetMergeRequestCommits(projectID, pr, opts)
	if err != nil {
		return nil, err
	}

	for _, prCommit := range prCommits {
		prReq := &PRCommit{
			ID:         prCommit.ID,
			Title:      prCommit.Title,
			AuthorName: prCommit.AuthorName,
			CreatedAt:  prCommit.CreatedAt,
		}
		respMRs = append(respMRs, prReq)
	}

	sort.SliceStable(respMRs, func(i, j int) bool { return respMRs[i].CreatedAt.Unix() > respMRs[j].CreatedAt.Unix() })
	if len(respMRs) > 0 {
		return respMRs[0], nil
	}

	return nil, nil
}

func getLatestPrNum(repo *types.Repository) int {
	var prNum int
	if len(repo.PRs) > 0 {
		prNum = repo.PRs[len(repo.PRs)-1]
	}
	return prNum
}

func getSubmissionID(codehostID int, commitID string) (string, error) {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return "", err
	}

	cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	changeInfo, err := cli.GetChangeDetail(commitID)
	if err != nil {
		return "", err
	}
	return changeInfo.SubmissionID, nil
}
