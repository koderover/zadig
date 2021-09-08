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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	githubservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/log"
)

type DownloadFromSourceArgs struct {
	CodehostID int    `json:"codeHostID"`
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
}

func DownloadFilesFromSource(args *DownloadFromSourceArgs, rootNameGetter func(afero.Fs) (string, error)) (fs.FS, error) {
	getter, err := getTreeGetter(args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get tree getter, err: %s", err)
		return nil, err
	}

	chartTree, err := getter.GetTreeContents(args.Owner, args.Repo, args.Path, args.Branch)
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
		f, _ := fs.ReadDir(afero.NewIOFS(chartTree), "")
		if len(f) == 1 {
			if err = chartTree.Rename(f[0].Name(), rootName); err != nil {
				log.Errorf("Failed to rename dir name from %s to %s, err: %s", f[0].Name(), rootName, err)
				return nil, err
			}
		}
	}

	return afero.NewIOFS(chartTree), nil
}

type treeGetter interface {
	GetTreeContents(owner, repo, path, branch string) (afero.Fs, error)
}

func getTreeGetter(codeHostID int) (treeGetter, error) {
	ch, err := codehost.GetCodeHostInfoByID(codeHostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", codeHostID, err)
		return nil, err
	}

	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr()), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.Address, ch.AccessToken)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}
