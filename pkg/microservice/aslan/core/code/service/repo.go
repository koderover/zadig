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

package service

import (
	"regexp"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/open"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
)

type RepoInfoList struct {
	Infos []*GitRepoInfo `json:"infos"`
}

type GitRepoInfo struct {
	Owner         string                `json:"repo_owner"`
	Namespace     string                `json:"repo_namespace"`
	Repo          string                `json:"repo"`
	CodehostID    int                   `json:"codehost_id"`
	Source        string                `json:"source"`
	DefaultBranch string                `json:"default_branch"`
	ErrorMsg      string                `json:"error_msg"` // get repo message fail message
	Branches      []*client.Branch      `json:"branches"`
	Tags          []*client.Tag         `json:"tags"`
	PRs           []*client.PullRequest `json:"prs"`
	ProjectUUID   string                `json:"project_uuid,omitempty"`
	RepoUUID      string                `json:"repo_uuid,omitempty"`
	RepoID        string                `json:"repo_id,omitempty"`
	Key           string                `json:"key"`
	// FilterRegexp is the regular expression filter for the branches and tags
	FilterRegexp string `json:"filter_regexp,omitempty"`
}

func (repo *GitRepoInfo) GetNamespace() string {
	if repo.Namespace == "" {
		return repo.Owner
	}
	return repo.Namespace
}

// ListRepoInfos ...
func ListRepoInfos(infos []*GitRepoInfo, log *zap.SugaredLogger) ([]*GitRepoInfo, error) {
	var wg sync.WaitGroup
	var errList *multierror.Error

	for _, info := range infos {
		ch, err := systemconfig.New().GetCodeHost(info.CodehostID)
		if err != nil {
			log.Errorf("get code host info err:%s", err)
			return nil, err
		}
		if ch.Type == setting.SourceFromOther {
			return []*GitRepoInfo{}, nil
		}
		codehostClient, err := open.OpenClient(ch, log)
		if err != nil {
			return nil, err
		}
		wg.Add(1)
		go func(info *GitRepoInfo) {
			defer func() {
				wg.Done()
			}()
			info.PRs, err = codehostClient.ListPrs(client.ListOpt{
				Namespace:   strings.Replace(info.GetNamespace(), "%2F", "/", -1),
				ProjectName: info.Repo,
			})
			if err != nil {
				errList = multierror.Append(errList, err)
				info.ErrorMsg = err.Error()
				info.PRs = []*client.PullRequest{}
				return
			}
		}(info)

		wg.Add(1)
		go func(info *GitRepoInfo) {
			defer func() {
				wg.Done()
			}()
			projectName := info.Repo
			if info.Source == CodeHostCodeHub {
				projectName = info.RepoUUID
			}
			info.Branches, err = codehostClient.ListBranches(client.ListOpt{
				Namespace:   strings.Replace(info.GetNamespace(), "%2F", "/", -1),
				ProjectName: projectName,
				Key:         info.Key,
			})
			if err != nil {
				errList = multierror.Append(errList, err)
				info.ErrorMsg = err.Error()
				info.Branches = []*client.Branch{}
				return
			}

		}(info)

		wg.Add(1)
		go func(info *GitRepoInfo) {
			defer func() {
				wg.Done()
			}()
			projectName := info.Repo
			if info.Source == CodeHostCodeHub {
				projectName = info.RepoID
			}

			info.Tags, err = codehostClient.ListTags(client.ListOpt{
				Namespace:   strings.Replace(info.GetNamespace(), "%2F", "/", -1),
				ProjectName: projectName,
				Key:         info.Key,
			})
			if err != nil {
				errList = multierror.Append(errList, err)
				info.ErrorMsg = err.Error()
				info.Tags = []*client.Tag{}
				return
			}
		}(info)
	}

	wg.Wait()
	for _, info := range infos {
		if info.FilterRegexp != "" {
			newBranchList := make([]*client.Branch, 0)
			for _, branch := range info.Branches {
				match, err := regexp.MatchString(info.FilterRegexp, branch.Name)
				if err != nil {
					log.Errorf("failed to match regular expression: %s on branch name :%s, error: %s", info.FilterRegexp, branch.Name, err)
					return nil, err
				}
				if match {
					newBranchList = append(newBranchList, branch)
				}
			}
			newTagList := make([]*client.Tag, 0)
			for _, tag := range info.Tags {
				match, err := regexp.MatchString(info.FilterRegexp, tag.Name)
				if err != nil {
					log.Errorf("failed to match regular expression: %s on tag name :%s, error: %s", info.FilterRegexp, tag.Name, err)
					return nil, err
				}
				if match {
					newTagList = append(newTagList, tag)
				}
			}
			info.Branches = newBranchList
			info.Tags = newTagList

			match, err := regexp.MatchString(info.FilterRegexp, info.DefaultBranch)
			if err != nil {
				log.Errorf("failed to compile regular expression: %s, the error is: %s", info.FilterRegexp, err)
				return nil, err
			}
			if !match {
				info.DefaultBranch = ""
			}
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Errorf("list repo info error: %v", err)
		return nil, err
	}
	return infos, nil
}

func MatchBranchesList(regular string, branches []string) []string {
	matchBranches := make([]string, 0)
	for _, branch := range branches {
		if matched, _ := regexp.MatchString(regular, branch); matched {
			matchBranches = append(matchBranches, branch)
		}
	}
	return matchBranches
}
