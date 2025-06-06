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

package fs

import (
	"fmt"
	"io/fs"

	"github.com/27149chen/afero"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	githubservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type DownloadFromSourceArgs struct {
	CodehostID int    `json:"codeHostID"`
	Owner      string `json:"owner"`
	Namespace  string `json:"namespace"`
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	RepoLink   string `json:"repoLink"`
}

func (args *DownloadFromSourceArgs) GetNamespace() string {
	if len(args.Namespace) > 0 {
		return args.Namespace
	}
	return args.Owner
}

func DownloadFileFromSource(args *DownloadFromSourceArgs) ([]byte, error) {
	getter, err := treeGetter(args.RepoLink, args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get tree getter, err: %s", err)
		return nil, err
	}
	owner := args.GetNamespace()
	return getter.GetFileContent(owner, args.Repo, args.Path, args.Branch)
}

func DownloadFilesFromSource(args *DownloadFromSourceArgs, rootNameGetter func(afero.Fs) (string, error)) (fs.FS, error) {
	getter, err := treeGetter(args.RepoLink, args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get tree getter, err: %s", err)
		return nil, err
	}
	owner := args.Namespace
	if owner == "" {
		owner = args.Owner
	}
	chartTree, err := getter.GetTreeContents(owner, args.Repo, args.Path, args.Branch)
	if err != nil {
		log.Errorf("Failed to get tree contents for service %+v, err: %s", args, err)
		return nil, err
	}

	rootName, err := rootNameGetter(chartTree)
	if err != nil {
		log.Errorf("Failed to get service name, err: %s", err)
		return nil, err
	}
	if rootName != "" {
		// rename the root path of the chart to the service name
		f, err := fs.ReadDir(afero.NewIOFS(chartTree), "")
		if err != nil {
			return nil, err
		}
		if len(f) == 1 {
			if err = chartTree.Rename(f[0].Name(), rootName); err != nil {
				log.Errorf("Failed to rename dir name from %s to %s, err: %s", f[0].Name(), rootName, err)
				return nil, err
			}
		}
	}

	return afero.NewIOFS(chartTree), nil
}

func treeGetter(repoLink string, codeHostID int) (TreeGetter, error) {
	if repoLink != "" {
		return GetPublicTreeGetter(repoLink)
	}

	return GetTreeGetter(codeHostID)
}

type TreeGetter interface {
	GetTreeContents(owner, repo, path, branch string) (afero.Fs, error)
	GetFileContent(owner, repo, path, branch string) ([]byte, error)
	GetTree(owner, repo, path, branch string) ([]*git.TreeNode, error)
	GetYAMLContents(owner, repo, path, branch string, isDir, split bool) ([]string, error)
}

func GetPublicTreeGetter(repoLink string) (TreeGetter, error) {
	return githubservice.NewClient("", config.ProxyHTTPSAddr(), true), nil
}

func GetTreeGetter(codeHostID int) (TreeGetter, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", codeHostID, err)
		return nil, err
	}

	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy, ch.DisableSSL)
	default:
		log.Errorf("GetTreeGetter invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}
