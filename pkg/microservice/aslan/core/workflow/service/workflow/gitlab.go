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

package workflow

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/ilyshin"
)

// RepoCommit : Repository commit struct
type RepoCommit struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	AuthorName string     `json:"author_name"`
	CreatedAt  *time.Time `json:"created_at"`
	Message    string     `json:"message"`
}

func QueryByBranch(id int, owner, name, branch string, log *zap.SugaredLogger) (*RepoCommit, error) {
	opt := &codehost.Option{
		CodeHostID: id,
	}
	ch, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, err
	}

	if ch.Type == setting.SourceFromGitlab {
		token, address := ch.AccessToken, ch.Address

		cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address))
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
		cli := gerrit.NewClient(ch.Address, ch.AccessToken)
		commit, err := cli.GetCommitByBranch(name, branch)
		if err != nil {
			return nil, err
		}

		commitDate, _ := time.Parse(time.RFC3339, commit.Author.Date)

		return &RepoCommit{
			ID:         commit.Commit,
			Title:      commit.Subject,
			AuthorName: commit.Author.Name,
			CreatedAt:  &commitDate,
			Message:    commit.Message,
		}, nil
	} else if ch.Type == setting.SourceFromIlyshin {
		client := ilyshin.NewClient(ch.Address, ch.AccessToken)
		branches, err := client.ListBranches(owner, name, log)
		if err != nil {
			return nil, err
		}
		for _, br := range branches {
			if br.Name == branch {
				return &RepoCommit{
					ID:         br.Commit.ID,
					Title:      br.Commit.Title,
					AuthorName: br.Commit.AuthorName,
					CreatedAt:  br.Commit.CreatedAt,
					Message:    br.Commit.Message,
				}, nil
			}
		}
	}

	return nil, errors.New(ch.Type + "is not supported yet")
}

func QueryByTag(id int, owner, name, tag string, log *zap.SugaredLogger) (*RepoCommit, error) {
	opt := &codehost.Option{
		CodeHostID: id,
	}
	ch, err := codehost.GetCodeHostInfo(opt)
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
		cli := gerrit.NewClient(ch.Address, ch.AccessToken)
		commit, err := cli.GetCommitByTag(name, tag)
		if err != nil {
			return nil, err
		}

		commitDate, _ := time.Parse(time.RFC3339, commit.Author.Date)

		return &RepoCommit{
			ID:         commit.Commit,
			Title:      commit.Subject,
			AuthorName: commit.Author.Name,
			CreatedAt:  &commitDate,
			Message:    commit.Message,
		}, nil
	} else if ch.Type == setting.SourceFromIlyshin {
		client := ilyshin.NewClient(ch.Address, ch.AccessToken)
		tags, err := client.ListTags(owner, name, log)
		if err != nil {
			return nil, err
		}
		for _, tagInfo := range tags {
			if tagInfo.Name == tag {
				return &RepoCommit{
					ID:         tagInfo.Commit.ID,
					Title:      tagInfo.Commit.Title,
					AuthorName: tagInfo.Commit.AuthorName,
					CreatedAt:  tagInfo.Commit.CreatedAt,
					Message:    tagInfo.Commit.Message,
				}, nil
			}
		}
	}

	return nil, errors.New(ch.Type + "is not supported yet")
}

type PRCommit struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	AuthorName  string     `json:"author_name"`
	CreatedAt   *time.Time `json:"created_at"`
	CheckoutRef string     `json:"checkout_ref"`
}

func GetLatestPrCommit(codehostID, pr int, namespace, projectName string, log *zap.SugaredLogger) (*PRCommit, error) {
	projectID := fmt.Sprintf("%s/%s", namespace, projectName)

	opt := &codehost.Option{
		CodeHostID: codehostID,
	}
	ch, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, err
	}

	token, address := ch.AccessToken, ch.Address
	cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address))
	if err != nil {
		return nil, fmt.Errorf("set base url failed, err:%v", err)
	}

	if ch.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken)
		change, err := cli.GetCurrentVersionByChangeID(projectName, pr)
		if err != nil {
			return nil, err
		}

		if _, ok := change.Revisions[change.CurrentRevision]; !ok {
			return nil, fmt.Errorf(
				"current version %s is not in revision map %v", change.CurrentRevision, change.Revisions)
		}

		tm, _ := time.Parse(gerrit.TimeFormat, change.Revisions[change.CurrentRevision].Created)
		return &PRCommit{
			ID:          change.CurrentRevision,
			Title:       change.Subject,
			AuthorName:  change.Revisions[change.CurrentRevision].Uploader.Name,
			CreatedAt:   &tm,
			CheckoutRef: change.Revisions[change.CurrentRevision].Ref,
		}, nil
	} else if ch.Type == setting.SourceFromIlyshin {
		client := ilyshin.NewClient(ch.Address, ch.AccessToken)
		commit, err := client.GetLatestPRCommitList(projectID, pr, log)
		if err != nil {
			return nil, err
		}
		return &PRCommit{
			ID:         commit.ID,
			Title:      commit.Title,
			AuthorName: commit.AuthorName,
			CreatedAt:  commit.CreatedAt,
		}, nil
	}
	return GetLatestPRCommitList(cli, projectID, pr)
}

func GetLatestPRCommitList(cli *gitlab.Client, projectID string, pr int) (*PRCommit, error) {
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
