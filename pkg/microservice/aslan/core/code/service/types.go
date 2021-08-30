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

	"github.com/andygrunwald/go-gerrit"
	"github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/pkg/tool/ilyshin"
	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/pkg/tool/codehub"
	gerrittool "github.com/koderover/zadig/pkg/tool/gerrit"
)

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
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
	RepoUUID      string `json:"repo_uuid,omitempty"`
	RepoID        string `json:"repo_id,omitempty"`
}

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

func ToBranches(obj interface{}) []*Branch {
	var res []*Branch

	switch os := obj.(type) {
	case []*github.Branch:
		for _, o := range os {
			res = append(res, &Branch{
				Name:      o.GetName(),
				Protected: o.GetProtected(),
			})
		}
	case []*gitlab.Branch:
		for _, o := range os {
			res = append(res, &Branch{
				Name:      o.Name,
				Protected: o.Protected,
				Merged:    o.Merged,
			})
		}
	case []string:
		for _, o := range os {
			res = append(res, &Branch{
				Name: o,
			})
		}
	case []*codehub.Branch:
		for _, o := range os {
			res = append(res, &Branch{
				Name:      o.Name,
				Protected: o.Protected,
				Merged:    o.Merged,
			})
		}
	case []*ilyshin.Branch:
		for _, o := range os {
			res = append(res, &Branch{
				Name:      o.Name,
				Protected: o.Protected,
				Merged:    o.Merged,
			})
		}
	}

	return res
}

func ToPullRequests(obj interface{}) []*PullRequest {
	var res []*PullRequest

	switch os := obj.(type) {
	case []*github.PullRequest:
		for _, o := range os {
			res = append(res, &PullRequest{
				ID:             o.GetNumber(),
				CreatedAt:      o.GetCreatedAt().Unix(),
				UpdatedAt:      o.GetUpdatedAt().Unix(),
				State:          o.GetState(),
				User:           o.GetUser().GetLogin(),
				Number:         o.GetNumber(),
				AuthorUsername: o.GetUser().GetLogin(),
				Title:          o.GetTitle(),
				SourceBranch:   o.GetHead().GetRef(),
				TargetBranch:   o.GetBase().GetRef(),
			})
		}
	case []*gitlab.MergeRequest:
		for _, o := range os {
			res = append(res, &PullRequest{
				ID:             o.IID,
				TargetBranch:   o.TargetBranch,
				SourceBranch:   o.SourceBranch,
				ProjectID:      o.ProjectID,
				Title:          o.Title,
				State:          o.State,
				CreatedAt:      o.CreatedAt.Unix(),
				UpdatedAt:      o.UpdatedAt.Unix(),
				AuthorUsername: o.Author.Username,
			})
		}
	case []*ilyshin.MergeRequest:
		for _, o := range os {
			res = append(res, &PullRequest{
				ID:             o.IID,
				TargetBranch:   o.TargetBranch,
				SourceBranch:   o.SourceBranch,
				ProjectID:      o.ProjectID,
				Title:          o.Title,
				State:          o.State,
				CreatedAt:      o.CreatedAt.Unix(),
				UpdatedAt:      o.UpdatedAt.Unix(),
				AuthorUsername: o.Author.Username,
			})
		}
	}

	return res
}

func ToNamespaces(obj interface{}) []*Namespace {
	var res []*Namespace

	switch os := obj.(type) {
	case *github.User:
		res = append(res, &Namespace{
			Name: os.GetLogin(),
			Path: os.GetLogin(),
			Kind: UserKind,
		})
	case []*github.User:
		for _, o := range os {
			res = append(res, &Namespace{
				Name: o.GetLogin(),
				Path: o.GetLogin(),
				Kind: UserKind,
			})
		}
	case []*github.Organization:
		for _, o := range os {
			res = append(res, &Namespace{
				Name: o.GetLogin(),
				Path: o.GetLogin(),
				Kind: OrgKind,
			})
		}
	case []*gitlab.Namespace:
		for _, o := range os {
			res = append(res, &Namespace{
				Name: o.Path,
				Path: o.FullPath,
				Kind: o.Kind,
			})
		}
	case []*codehub.Namespace:
		for _, o := range os {
			res = append(res, &Namespace{
				Name:        o.Name,
				Path:        o.Path,
				Kind:        o.Kind,
				ProjectUUID: o.ProjectUUID,
			})
		}
	case []*ilyshin.Project:
		for _, o := range os {
			res = append(res, &Namespace{
				Name: o.Namespace.Name,
				Path: o.Namespace.Path,
				Kind: GroupKind,
			})
		}
	}

	return res
}

func ToProjects(obj interface{}) []*Project {
	var res []*Project

	switch os := obj.(type) {
	case []*github.Repository:
		for _, o := range os {
			res = append(res, &Project{
				ID:            int(o.GetID()),
				Name:          o.GetName(),
				DefaultBranch: o.GetDefaultBranch(),
				Namespace:     o.GetOwner().GetLogin(),
			})
		}
	case []*gitlab.Project:
		for _, o := range os {
			res = append(res, &Project{
				ID:            o.ID,
				Name:          o.Path,
				Namespace:     o.Namespace.FullPath,
				Description:   o.Description,
				DefaultBranch: o.DefaultBranch,
			})
		}
	case []*gerrit.ProjectInfo:
		for ind, o := range os {
			res = append(res, &Project{
				ID:            ind,                       // fake id
				Name:          gerrittool.Unescape(o.ID), // id could have %2F
				Description:   o.Description,
				DefaultBranch: "master",
				Namespace:     gerrittool.DefaultNamespace,
			})
		}
	case []*codehub.Project:
		for _, project := range os {
			res = append(res, &Project{
				Name:          project.Name,
				Description:   project.Description,
				DefaultBranch: project.DefaultBranch,
				Namespace:     project.Namespace,
				RepoUUID:      project.RepoUUID,
				RepoID:        project.RepoID,
			})
		}
	case []*ilyshin.Project:
		for _, o := range os {
			namespaces := strings.Split(o.PathWithNamespace, "/")
			res = append(res, &Project{
				ID:            o.ID,
				Name:          o.Path,
				Namespace:     namespaces[0],
				Description:   o.Description,
				DefaultBranch: "master",
			})
		}
	}

	return res
}

func ToTags(obj interface{}) []*Tag {
	var res []*Tag

	switch os := obj.(type) {
	case []*github.RepositoryTag:
		for _, o := range os {
			res = append(res, &Tag{
				Name:       o.GetName(),
				ZipballURL: o.GetZipballURL(),
				TarballURL: o.GetTarballURL(),
			})
		}
	case []*gitlab.Tag:
		for _, o := range os {
			res = append(res, &Tag{
				Name:    o.Name,
				Message: o.Message,
			})
		}
	case []*gerrit.TagInfo:
		for _, o := range os {
			res = append(res, &Tag{
				Name:    o.Ref,
				Message: o.Message,
			})
		}
	case []*codehub.Tag:
		for _, o := range os {
			res = append(res, &Tag{
				Name: o.Name,
			})
		}
	case []*ilyshin.Tag:
		for _, o := range os {
			res = append(res, &Tag{
				Name:    o.Name,
				Message: o.Message,
			})
		}
	}

	return res
}
