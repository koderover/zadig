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

package types

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Repository struct
type Repository struct {
	Source        string `bson:"source,omitempty"          json:"source,omitempty"         yaml:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"               yaml:"repo_owner"`
	RepoNamespace string `bson:"repo_namespace"            json:"repo_namespace"           yaml:"repo_namespace"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"                yaml:"repo_name"`
	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"    yaml:"remote_name,omitempty"`
	Branch        string `bson:"branch"                    json:"branch"                   yaml:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"             yaml:"pr,omitempty"`
	PRs           []int  `bson:"prs,omitempty"             json:"prs,omitempty"            yaml:"prs,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"            yaml:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"      yaml:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty" yaml:"commit_message,omitempty"`
	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"  yaml:"checkout_path,omitempty"`
	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"     yaml:"submodules,omitempty"`
	// UseDefault defines if the repo can be configured in start pipeline task page
	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"    yaml:"use_default,omitempty"`
	// IsPrimary used to generated image and package name, each build has one primary repo
	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"               yaml:"is_primary"`
	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"              yaml:"codehost_id"`
	// add
	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"             yaml:"oauth_token"`
	Address     string `bson:"address"                      json:"address"                 yaml:"address"`
	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"   yaml:"author_name,omitempty"`
	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"  yaml:"checkout_ref,omitempty"`
	// codehub
	ProjectUUID string `bson:"project_uuid,omitempty"       json:"project_uuid,omitempty"  yaml:"project_uuid,omitempty"`
	RepoUUID    string `bson:"repo_uuid,omitempty"          json:"repo_uuid,omitempty"     yaml:"repo_uuid,omitempty"`
	RepoID      string `bson:"repo_id,omitempty"            json:"repo_id,omitempty"       yaml:"repo_id,omitempty"`
	Username    string `bson:"username,omitempty"           json:"username,omitempty"      yaml:"username,omitempty"`
	Password    string `bson:"password,omitempty"           json:"password,omitempty"      yaml:"password,omitempty"`
	// Now EnableProxy is not something we store. We decide this on runtime
	EnableProxy bool `bson:"-"       json:"enable_proxy,omitempty"                         yaml:"enable_proxy,omitempty"`
	// FilterRegexp is the regular expression filter for the branches and tags
	FilterRegexp string `bson:"-"    json:"filter_regexp,omitempty"                        yaml:"filter_regexp,omitempty"`
	// The address of the code base input of the other type
	AuthType           AuthType `bson:"auth_type,omitempty"             json:"auth_type,omitempty"               yaml:"auth_type,omitempty"`
	SSHKey             string   `bson:"ssh_key,omitempty"               json:"ssh_key,omitempty"                 yaml:"ssh_key,omitempty"`
	PrivateAccessToken string   `bson:"private_access_token,omitempty"  json:"private_access_token,omitempty"    yaml:"private_access_token,omitempty"`
}

type BranchFilterInfo struct {
	// repository identifier
	CodehostID    int    `bson:"codehost_id"  json:"codehost_id"`
	RepoOwner     string `bson:"repo_owner"   json:"repo_owner"`
	RepoName      string `bson:"repo_name"    json:"repo_name"`
	RepoNamespace string `bson:"repo_namespace" json:"repo_namespace"`
	// actual regular expression filter
	FilterRegExp  string `bson:"filter_regexp"  json:"filter_regexp"`
	DefaultBranch string `bson:"default_branch" json:"default_branch"`
}

func (bf *BranchFilterInfo) GetNamespace() string {
	if len(bf.RepoNamespace) > 0 {
		return bf.RepoNamespace
	}
	return bf.RepoOwner
}

// GetReleaseCandidateTag 返回待发布对象Tag
// Branch: 20060102150405-{TaskID}-master
// PR: 20060102150405-{TaskID}-pr-1765
// Branch + PR: 20060102150405-{TaskID}-master-pr-1276
// Tag: 20060102150405-{TaskID}-v0.9.1
func (repo *Repository) GetReleaseCandidateTag(taskID int64) string {

	var tag string

	timeStamp := time.Now().Format("20060102150405")

	if repo.Tag != "" {
		tag = fmt.Sprintf("%s-%d-%s", timeStamp, taskID, repo.Tag)
	} else if repo.Branch != "" && repo.PR != 0 {
		tag = fmt.Sprintf("%s-%d-%s-pr-%d", timeStamp, taskID, repo.Branch, repo.PR)
	} else if repo.Branch == "" && repo.PR != 0 {
		tag = fmt.Sprintf("%s-%d-pr-%d", timeStamp, taskID, repo.PR)
	} else if repo.Branch != "" && repo.PR == 0 {
		tag = fmt.Sprintf("%s-%d-%s", timeStamp, taskID, repo.Branch)
	} else {
		return "invalid"
	}

	// 验证tag是否符合kube的image tag规则
	reg := regexp.MustCompile(`[^\w.-]`)
	tagByte := reg.ReplaceAll([]byte(tag), []byte("-"))
	tag = string(tagByte)
	if len(tag) > 127 {
		return "invalid"
	}
	return tag
}

func (repo *Repository) GetRepoNamespace() string {
	if repo.RepoNamespace != "" {
		return repo.RepoNamespace
	}
	return repo.RepoOwner
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

	// ProviderGiteeEE
	ProviderGiteeEE = "gitee-enterprise"

	// ProviderOther
	ProviderOther = "other"
)

// PRRef returns refs format
// It will check repo provider type, by default returns github refs format.
//
// e.g. github returns refs/pull/1/head
// e.g. gitlab returns merge-requests/1/head
func (r *Repository) PRRef() string {
	if strings.ToLower(r.Source) == ProviderGitlab || strings.ToLower(r.Source) == ProviderCodehub {
		return fmt.Sprintf("merge-requests/%d/head", r.PR)
	} else if strings.ToLower(r.Source) == ProviderGerrit {
		return r.CheckoutRef
	}
	return fmt.Sprintf("refs/pull/%d/head", r.PR)
}

func (r *Repository) PRRefByPRID(pr int) string {
	if strings.ToLower(r.Source) == ProviderGitlab || strings.ToLower(r.Source) == ProviderCodehub {
		return fmt.Sprintf("merge-requests/%d/head", pr)
	} else if strings.ToLower(r.Source) == ProviderGerrit {
		return r.CheckoutRef
	}
	return fmt.Sprintf("refs/pull/%d/head", pr)
}

// BranchRef returns branch refs format
// e.g. refs/heads/master
func (r *Repository) BranchRef() string {
	return fmt.Sprintf("refs/heads/%s", r.Branch)
}

// TagRef returns the tag ref of current repo
// e.g. refs/tags/v1.0.0
func (r *Repository) TagRef() string {
	return fmt.Sprintf("refs/tags/%s", r.Tag)
}

// Ref returns the changes ref of current repo in the following order:
// 1. tag ref
// 2. branch ref
// 3. pr ref
func (r *Repository) Ref() string {
	if len(r.Tag) > 0 {
		return r.TagRef()
	} else if len(r.Branch) > 0 {
		return r.BranchRef()
	} else if r.PR > 0 {
		return r.PRRef()
	}

	return ""
}
