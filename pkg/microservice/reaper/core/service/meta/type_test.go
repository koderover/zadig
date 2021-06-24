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

package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProxyUrl(t *testing.T) {
	proxy := &Proxy{
		Type:     "http",
		Username: "foo",
		Password: "password",
		Address:  "os.koderover.com",
		Port:     3032,
	}
	for _, test := range []*struct {
		needPassword bool
		expected     string
	}{
		{
			needPassword: true,
			expected:     "http://foo:password@os.koderover.com:3032",
		},
		{
			needPassword: false,
			expected:     "http://os.koderover.com:3032",
		},
	} {
		proxy.NeedPassword = test.needPassword
		assert.Equal(t, test.expected, proxy.GetProxyURL())
	}
}

func TestGetGitlabHost(t *testing.T) {
	for _, test := range []*Git{
		{
			GitlabHost: "gitlab.test.com",
		},
		{},
	} {
		if test.GitlabHost == "" {
			assert.Equal(t, defaultGitlabHost, test.GetGitlabHost())
		} else {
			assert.Equal(t, test.GitlabHost, test.GetGitlabHost())
		}
	}
}

func TestSSHCloneURL(t *testing.T) {
	assert := assert.New(t)

	git := &Git{
		GithubHost: "github.com",
		GitlabHost: "gitlab.company.io",
	}
	assert.Equal("git@github.com:koderover/zadig.git", git.SSHCloneURL(ProviderGithub, "koderover", "zadig"))
	assert.Equal("git@gitlab.company.io:koderover/zadig-X.git", git.SSHCloneURL(ProviderGitlab, "koderover", "zadig-X"))
}

func TestHTTPSCloneURL(t *testing.T) {
	assert := assert.New(t)

	git := &Git{
		GithubHost: "github.com",
		GitlabHost: "gitlab.company.io",
	}

	assert.Equal(
		"https://x-access-token:token@github.com/koderover/zadig.git",
		git.HTTPSCloneURL(ProviderGithub, "token", "koderover", "zadig"))
	assert.Equal(
		"https://gitlab.company.io/koderover/zadig-X.git",
		git.HTTPSCloneURL(ProviderGitlab, "", "koderover", "zadig-X"))
}
