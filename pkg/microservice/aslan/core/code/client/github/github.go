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

package github

import (
	"context"
	"fmt"
	"regexp"

	github2 "github.com/google/go-github/v35/github"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/tool/git/github"
)

type Config struct {
	AccessToken string `json:"access_token"`
	EnableProxy bool   `json:"enable_proxy"`
}

type Client struct {
	Client *github.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {

	cfg := &github.Config{
		AccessToken: c.AccessToken,
	}
	if c.EnableProxy {
		cfg.Proxy = config.ProxyHTTPSAddr()
	}
	return &Client{
		Client: github.NewClient(cfg),
	}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(context.TODO(), opt.Namespace, opt.ProjectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		matched, _ := regexp.MatchString(fmt.Sprintf(`%s`, opt.Key), o.GetName())
		if matched {
			res = append(res, &client.Branch{
				Name:      o.GetName(),
				Protected: o.GetProtected(),
			})
		}
	}
	return res, nil
}

func (c *Client) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	tags, err := c.Client.ListTags(context.TODO(), opt.Namespace, opt.ProjectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Tag
	for _, o := range tags {
		matched, _ := regexp.MatchString(fmt.Sprintf(`%s`, opt.Key), o.GetName())
		if matched {
			res = append(res, &client.Tag{
				Name:       o.GetName(),
				ZipballURL: o.GetZipballURL(),
				TarballURL: o.GetTarballURL(),
			})
		}
	}
	return res, nil
}

func (c *Client) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	prs, err := c.Client.ListPullRequests(context.TODO(), opt.Namespace, opt.ProjectName, &github2.PullRequestListOptions{
		ListOptions: github2.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
	}
	var res []*client.PullRequest
	for _, o := range prs {
		res = append(res, &client.PullRequest{
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
	return res, nil
}

func (c *Client) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	user, err := c.Client.GetAuthenticatedUser(context.TODO())
	if err != nil {
		return nil, err
	}
	namespaceUser := client.Namespace{
		Name: user.GetLogin(),
		Path: user.GetLogin(),
		Kind: client.UserKind,
	}

	organizations, err := c.Client.ListOrganizationsForAuthenticatedUser(context.TODO(), nil)
	if err != nil {
		return nil, err
	}

	var res []*client.Namespace
	res = append(res, &namespaceUser)
	for _, o := range organizations {
		res = append(res, &client.Namespace{
			Name: o.GetLogin(),
			Path: o.GetLogin(),
			Kind: client.OrgKind,
		})
	}
	return res, nil
}

func (c *Client) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	repos, err := c.Client.ListRepositoriesForAuthenticatedUser(context.TODO(), opt.Namespace, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Project
	for _, o := range repos {
		// Note. when using gitHub api to filter repos, private repo will not be returned
		// we need to filter the repos with query namespace when repo list is returned
		if len(opt.Namespace) > 0 && o.GetOwner().GetLogin() != opt.Namespace {
			continue
		}
		res = append(res, &client.Project{
			ID:            int(o.GetID()),
			Name:          o.GetName(),
			DefaultBranch: o.GetDefaultBranch(),
			Namespace:     o.GetOwner().GetLogin(),
		})
	}
	return res, nil
}

func (c *Client) ListCommits(opt client.ListOpt) ([]*client.Commit, error) {
	commits, err := c.Client.ListCommitsForBranch(context.TODO(), opt.Namespace, opt.ProjectName, opt.TargetBranch, &github.ListOptions{
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
			ID:      *c.SHA,
			Message: *c.Commit.Message,
			Author:  *c.Commit.Author.Name,
			// GitHub won't give creation timestamp in listing API, we will have to set it to 0.
			CreatedAt: 0,
		})
	}
	return res, nil
}
