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

package gitlab

import (
	"go.uber.org/zap"

	gogitlab "github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
)

type Config struct {
	Address     string `json:"address"`
	AccessToken string `json:"access_token"`
	// the field determine whether the proxy is enabled
	EnableProxy bool `json:"enable_proxy"`
	DisableSSL  bool `json:"disable_ssl"`
}

type Client struct {
	Client *gitlab.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {

	client, err := gitlab.NewClient(id, c.Address, c.AccessToken, config.ProxyHTTPSAddr(), c.EnableProxy, c.DisableSSL)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(opt.Namespace, opt.ProjectName, opt.Key, &gitlab.ListOptions{
		Page:          opt.Page,
		PerPage:       opt.PerPage,
		NoPaginated:   true,
		MatchBranches: opt.MatchBranches,
	})
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, b := range bList {
		res = append(res, &client.Branch{
			Name:      b.Name,
			Protected: b.Protected,
			Merged:    b.Merged,
		})
	}
	return res, nil
}

func (c *Client) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	tags, err := c.Client.ListTags(opt.Namespace, opt.ProjectName, &gitlab.ListOptions{
		Page:        opt.Page,
		PerPage:     opt.PerPage,
		NoPaginated: true,
	}, opt.Key)
	if err != nil {
		return nil, err
	}
	var res []*client.Tag
	for _, o := range tags {
		res = append(res, &client.Tag{
			Name:    o.Name,
			Message: o.Message,
		})
	}
	return res, nil
}

func (c *Client) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	prs, err := c.Client.ListOpenedProjectMergeRequests(opt.Namespace, opt.ProjectName, opt.TargetBranch, opt.Key, &gitlab.ListOptions{
		Page:        opt.Page,
		PerPage:     opt.PerPage,
		NoPaginated: true,
	})
	if err != nil {
		return nil, err
	}
	var res []*client.PullRequest
	for _, o := range prs {
		res = append(res, &client.PullRequest{
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
	return res, nil
}

func (c *Client) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	nsList, err := c.Client.ListNamespaces(keyword, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Namespace
	for _, o := range nsList {
		res = append(res, &client.Namespace{
			Name: o.Path,
			Path: o.FullPath,
			Kind: o.Kind,
		})
	}
	return res, nil
}

func (c *Client) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	var projects []*gogitlab.Project
	var err error
	switch opt.NamespaceType {
	case client.GroupKind:
		projects, err = c.Client.ListGroupProjects(opt.Namespace, opt.Key, &gitlab.ListOptions{
			Page:        opt.Page,
			PerPage:     opt.PerPage,
			NoPaginated: true,
		})
		if err != nil {
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}
	default:
		projects, err = c.Client.ListUserProjects(opt.Namespace, opt.Key, &gitlab.ListOptions{
			Page:        opt.Page,
			PerPage:     opt.PerPage,
			NoPaginated: true,
		})
		if err != nil {
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}

	}
	var res []*client.Project
	for _, o := range projects {
		res = append(res, &client.Project{
			ID:            o.ID,
			Name:          o.Path,
			Namespace:     o.Namespace.FullPath,
			Description:   o.Description,
			DefaultBranch: o.DefaultBranch,
		})
	}
	return res, nil
}

func (c *Client) ListCommits(opt client.ListOpt) ([]*client.Commit, error) {
	commits, err := c.Client.ListCommits(opt.Namespace, opt.ProjectName, opt.TargetBranch, &gitlab.ListOptions{
		Page:        opt.Page,
		PerPage:     opt.PerPage,
		NoPaginated: true,
	})
	if err != nil {
		return nil, e.ErrCodehostListCommits.AddDesc(err.Error())
	}
	var res []*client.Commit
	for _, c := range commits {
		res = append(res, &client.Commit{
			ID:        c.ID,
			Message:   c.Message,
			CreatedAt: c.CreatedAt.Unix(),
			Author:    c.AuthorName,
		})
	}
	return res, nil
}
