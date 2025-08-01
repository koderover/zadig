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

package client

const (
	OrgKind        = "org"
	GroupKind      = "group"
	UserKind       = "user"
	EnterpriseKind = "enterprise"
)

type CodeHostClient interface {
	ListBranches(opt ListOpt) ([]*Branch, error)
	ListTags(opt ListOpt) ([]*Tag, error)
	ListPrs(opt ListOpt) ([]*PullRequest, error)
	ListNamespaces(keyword string) ([]*Namespace, error)
	ListProjects(opt ListOpt) ([]*Project, error)
	ListCommits(opt ListOpt) ([]*Commit, error)
}

type ListOpt struct {
	Namespace     string
	NamespaceType string
	ProjectName   string
	Key           string
	Page          int
	PerPage       int
	TargetBranch  string
	MatchBranches bool
}

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
}

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

type PullRequest struct {
	ID             int    `json:"id"`
	TargetBranch   string `json:"targetBranch"`
	SourceBranch   string `json:"sourceBranch"`
	ProjectID      int    `json:"projectId"`
	Title          string `json:"title"`
	State          string `json:"state"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	AuthorUsername string `json:"authorUsername"`
	Number         int    `json:"number"`
	User           string `json:"user"`
	Base           string `json:"base,omitempty"`
}

type Namespace struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	ProjectUUID string `json:"project_uuid,omitempty"`
}

type Project struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	Namespace     string `json:"namespace"`
	RepoID        string `json:"repo_id,omitempty"`
}

type Commit struct {
	ID        string `json:"commit_id"`
	Message   string `json:"commit_message"`
	Author    string `json:"author"`
	CreatedAt int64  `json:"created_at"`
}
