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

package gitee

import (
	"context"

	giteeClient "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
)

type EEConfig struct {
	AccessToken string `json:"access_token"`
	EnableProxy bool   `json:"enable_proxy"`
	Address     string `json:"address"`
}

type EEClient struct {
	Client      *gitee.Client
	AccessToken string
	Address     string
}

func (c *EEConfig) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	client := gitee.NewClient(id, c.Address, c.AccessToken, config.ProxyHTTPSAddr(), c.EnableProxy)
	return &EEClient{
		Client:      client,
		AccessToken: c.AccessToken,
		Address:     c.Address,
	}, nil
}

func (c *EEClient) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(context.TODO(), opt.Namespace, opt.ProjectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name:      o.Name,
			Protected: o.Protected,
		})
	}
	return res, nil
}

func (c *EEClient) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	tags, err := c.Client.ListTags(context.TODO(), c.Address, c.AccessToken, opt.Namespace, opt.ProjectName)
	if err != nil {
		return nil, err
	}
	var resp []*client.Tag
	for _, tag := range tags {
		resp = append(resp, &client.Tag{
			Name:    tag.Name,
			Message: tag.Message,
		})
	}

	return resp, nil
}

func (c *EEClient) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	prs, err := c.Client.ListPullRequests(context.TODO(), opt.Namespace, opt.ProjectName, &giteeClient.GetV5ReposOwnerRepoPullsOpts{
		PerPage: optional.NewInt32(100),
	})
	if err != nil {
		return nil, err
	}
	var res []*client.PullRequest
	for _, o := range prs {
		res = append(res, &client.PullRequest{
			ID:             int(o.Number),
			State:          o.State,
			User:           o.User.Login,
			Number:         int(o.Number),
			AuthorUsername: o.User.Login,
			Title:          o.Title,
			SourceBranch:   o.Base.Ref,
			TargetBranch:   o.Base.Ref,
		})
	}
	return res, nil
}

func (c *EEClient) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	// since there are enterprise AND organization simultaneously, we need to list both
	enterprises, err := c.Client.ListEnterprises(context.TODO())
	if err != nil {
		return nil, err
	}
	organizations, err := c.Client.ListOrganizationsForAuthenticatedUser(context.TODO())
	if err != nil {
		return nil, err
	}

	var res []*client.Namespace
	for _, e := range enterprises {
		res = append(res, &client.Namespace{
			Name: e.Name,
			Path: e.Path,
			Kind: client.EnterpriseKind,
		})
	}
	for _, o := range organizations {
		res = append(res, &client.Namespace{
			Name: o.Login,
			Path: o.Login,
			Kind: client.OrgKind,
		})
	}
	return res, nil
}

func (c *EEClient) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	var projects []gitee.Project
	var err error
	switch opt.NamespaceType {
	case client.EnterpriseKind:
		projects, err = c.Client.ListRepositoryForEnterprise(c.Address, c.AccessToken, opt.Namespace, opt.Key, opt.Page, opt.PerPage)
		if err != nil {
			return nil, err
		}
	case client.OrgKind:
		projects, err = c.Client.ListRepositoriesForOrg(c.Address, c.AccessToken, opt.Namespace, opt.Page, opt.PerPage)
		if err != nil {
			return nil, err
		}
	default:
		projects, err = c.Client.ListRepositoriesForAuthenticatedUser(c.Address, c.AccessToken, opt.Key, opt.Page, opt.PerPage)
		if err != nil {
			return nil, err
		}
	}

	var res []*client.Project
	for _, project := range projects {
		res = append(res, &client.Project{
			ID:            project.ID,
			Name:          project.Name,
			RepoID:        project.Path,
			Namespace:     project.Namespace.Path,
			DefaultBranch: project.DefaultBranch,
		})
	}

	return res, nil
}

func (c *EEClient) ListCommits(opt client.ListOpt) ([]*client.Commit, error) {
	return make([]*client.Commit, 0), nil
}
