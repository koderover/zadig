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

type CodeHostClient interface {
	ListBranches(opt ListOpt) ([]*Branch, error)
	ListTags(opt ListOpt) ([]*Tag, error)
	ListPrs(opt ListOpt) ([]*PullRequest, error)
}

type ListOpt struct {
	Namespace   string
	ProjectName string
	Key         string
	Page        int
	PerPage     int
	TargeBr     string
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
