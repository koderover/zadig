/*
Copyright 2022 The KodeRover Authors.

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

package step

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/types"
)

type StepGitSpec struct {
	Repos []*Repo `yaml:"repos"`
	Proxy *Proxy  `yaml:"proxy"`
}

type Repo struct {
	Source             string         `yaml:"source"`
	Address            string         `yaml:"address"`
	CodeHostID         int            `yaml:"code_host_id"`
	Owner              string         `yaml:"owner"`
	Name               string         `yaml:"name"`
	Namespace          string         `yaml:"namespace"`
	RemoteName         string         `yaml:"remote_name"`
	Branch             string         `yaml:"branch"`
	PR                 int            `yaml:"pr"`
	Tag                string         `yaml:"tag"`
	CheckoutPath       string         `yaml:"checkout_path"`
	SubModules         bool           `yaml:"submodules"`
	OauthToken         string         `yaml:"oauthToken"`
	User               string         `yaml:"username"`
	Password           string         `yaml:"password"`
	CheckoutRef        string         `yaml:"checkout_ref"`
	EnableProxy        bool           `yaml:"enable_proxy"`
	AuthType           types.AuthType `yaml:"auth_type,omitempty"`
	SSHKey             string         `yaml:"ssh_key,omitempty"`
	PrivateAccessToken string         `yaml:"private_access_token,omitempty"`
}

const (
	// ProviderGithub ...
	ProviderGithub = "github"
	// ProviderGitlab ...
	ProviderGitlab = "gitlab"

	// ProviderGerrit
	ProviderGerrit = "gerrit"

	// ProviderCodehub
	ProviderCodehub = "codehub"

	// ProviderGitee
	ProviderGitee = "gitee"

	// ProviderOther
	ProviderOther = "other"

	//	Oauth prefix
	OauthTokenPrefix = "oauth2"

	FileName = "reaper.tar.gz"
)

func (p *Proxy) GetProxyURL() string {
	var uri string
	if p.NeedPassword {
		uri = fmt.Sprintf("%s://%s:%s@%s:%d",
			p.Type,
			p.Username,
			p.Password,
			p.Address,
			p.Port,
		)
		return uri
	}

	uri = fmt.Sprintf("%s://%s:%d",
		p.Type,
		p.Address,
		p.Port,
	)
	return uri
}

// PRRef returns refs format
// It will check repo provider type, by default returns github refs format.
//
// e.g. github returns refs/pull/1/head
// e.g. gitlab returns merge-requests/1/head
func (r *Repo) PRRef() string {
	if strings.ToLower(r.Source) == ProviderGitlab || strings.ToLower(r.Source) == ProviderCodehub {
		return fmt.Sprintf("merge-requests/%d/head", r.PR)
	} else if strings.ToLower(r.Source) == ProviderGerrit {
		return r.CheckoutRef
	}
	return fmt.Sprintf("refs/pull/%d/head", r.PR)
}

// BranchRef returns branch refs format
// e.g. refs/heads/master
func (r *Repo) BranchRef() string {
	return fmt.Sprintf("refs/heads/%s", r.Branch)
}

// TagRef returns the tag ref of current repo
// e.g. refs/tags/v1.0.0
func (r *Repo) TagRef() string {
	return fmt.Sprintf("refs/tags/%s", r.Tag)
}

// Ref returns the changes ref of current repo in the following order:
// 1. tag ref
// 2. branch ref
// 3. pr ref
func (r *Repo) Ref() string {
	if len(r.Tag) > 0 {
		return r.TagRef()
	} else if len(r.Branch) > 0 {
		return r.BranchRef()
	} else if r.PR > 0 {
		return r.PRRef()
	}

	return ""
}
