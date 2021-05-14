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
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type RepoInfoList struct {
	Infos []*GitRepoInfo `json:"infos"`
}

type GitRepoInfo struct {
	Owner         string           `json:"repo_owner"`
	Repo          string           `json:"repo"`
	CodehostID    int              `json:"codehost_id"`
	Source        string           `json:"source"`
	DefaultBranch string           `json:"default_branch"`
	ErrorMsg      string           `json:"error_msg"` // repo信息是否拉取成功
	Branches      []*gitlab.Branch `json:"branches"`
	Tags          []*gitlab.Tag    `json:"tags"`
	//Releases      []*GitRelease `json:"releases"`
	PRs []*gitlab.MergeRequest `json:"prs"`
}

// ListRepoInfos ...
func ListRepoInfos(infos []*GitRepoInfo, param string, log *xlog.Logger) ([]*GitRepoInfo, error) {
	var err error
	var wg sync.WaitGroup
	var errList *multierror.Error

	for _, info := range infos {
		//pb 代表pr and branch
		if param == "" || param == "bp" {
			wg.Add(1)
			go func(info *GitRepoInfo) {
				defer func() {
					wg.Done()
				}()
				info.PRs, err = CodehostListPRs(info.CodehostID, info.Repo, strings.Replace(info.Owner, "%2F", "/", -1), "", log)
				if err != nil {
					errList = multierror.Append(errList, err)
					info.ErrorMsg = err.Error()
					info.PRs = []*gitlab.MergeRequest{}
					return
				}
			}(info)
		}

		wg.Add(1)
		go func(info *GitRepoInfo) {
			defer func() {
				wg.Done()
			}()
			info.Branches, err = CodehostListBranches(info.CodehostID, info.Repo, strings.Replace(info.Owner, "%2F", "/", -1), log)
			if err != nil {
				errList = multierror.Append(errList, err)
				info.ErrorMsg = err.Error()
				info.Branches = []*gitlab.Branch{}
				return
			}

		}(info)

		//bt 代表branch and tag
		if param == "" || param == "bt" {
			wg.Add(1)
			go func(info *GitRepoInfo) {
				defer func() {
					wg.Done()
				}()
				info.Tags, err = CodehostListTags(info.CodehostID, info.Repo, strings.Replace(info.Owner, "%2F", "/", -1), log)
				if err != nil {
					errList = multierror.Append(errList, err)
					info.ErrorMsg = err.Error()
					info.Tags = []*gitlab.Tag{}
					return
				}
			}(info)
		}
	}

	wg.Wait()
	if err := errList.ErrorOrNil(); err != nil {
		log.Errorf("list repo info error: %v", err)
	}
	return infos, nil
}
