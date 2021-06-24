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

package reaper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
)

func TestReaper_BuildGitCommands_EmptyRepoName(t *testing.T) {
	r := createReaperForTest(t, NewGoCacheManager())
	repo := &meta.Repo{
		Source: meta.ProviderGerrit,
	}
	cmds := r.buildGitCommands(repo)
	assert.Len(t, cmds, 0)
}

func TestReaper_BuildGitCommands_WithBranch(t *testing.T) {
	r := createReaperForTest(t, NewGoCacheManager())
	r.Ctx.Git = &meta.Git{
		GithubHost: "github.com",
	}

	repo := &meta.Repo{
		Source:     meta.ProviderGithub,
		Owner:      "koderover",
		Name:       "zadig",
		RemoteName: "origin",
		Branch:     "master",
		OauthToken: "token",
	}
	cmds := r.buildGitCommands(repo)

	assert := assert.New(t)
	assert.Equal([]string{"git", "init"}, cmds[0].Cmd.Args)
	assert.Equal([]string{"git", "remote", "add", "origin", "https://x-access-token:token@github.com/koderover/zadig.git"}, cmds[1].Cmd.Args)
	assert.Equal([]string{"git", "fetch", "origin", "+refs/heads/master", "--depth=1"}, cmds[2].Cmd.Args)
	assert.Equal([]string{"git", "checkout", "-qf", "FETCH_HEAD"}, cmds[3].Cmd.Args)
}

func TestReaper_BuildGitCommands_WithTag(t *testing.T) {
	r := createReaperForTest(t, NewGoCacheManager())
	r.Ctx.Git = &meta.Git{
		GithubHost: "github.com",
	}

	repo := &meta.Repo{
		Source:     meta.ProviderGithub,
		Owner:      "koderover",
		Name:       "zadig",
		RemoteName: "origin",
		Branch:     "master",
		Tag:        "aslan",
		OauthToken: "token",
	}
	cmds := r.buildGitCommands(repo)

	assert := assert.New(t)
	assert.Equal([]string{"git", "init"}, cmds[0].Cmd.Args)
	assert.Equal([]string{"git", "remote", "add", "origin", "https://x-access-token:token@github.com/koderover/zadig.git"}, cmds[1].Cmd.Args)
	assert.Equal([]string{"git", "checkout", "-qf", "FETCH_HEAD"}, cmds[3].Cmd.Args)
}
