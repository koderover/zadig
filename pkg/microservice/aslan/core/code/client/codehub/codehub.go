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

package codehub

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type Config struct {
	Region      string `json:"region"`
	AccessKey   string `json:"application_id"`
	SecretKey   string `json:"client_secret"`
	EnableProxy bool   `json:"enable_proxy"`
}

type Client struct {
	Client *codehub.CodeHubClient
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	codehubClient := codehub.NewCodeHubClient(c.AccessKey, c.SecretKey, c.Region, config.ProxyHTTPSAddr(), c.EnableProxy)
	return &Client{Client: codehubClient}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.BranchList(opt.ProjectName)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name:      o.Name,
			Protected: o.Protected,
			Merged:    o.Merged,
		})
	}
	return res, nil
}

func (c *Client) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	tags, err := c.Client.TagList(opt.ProjectName)
	if err != nil {
		return nil, err
	}

	var res []*client.Tag
	for _, o := range tags {
		res = append(res, &client.Tag{
			Name: o.Name,
		})
	}

	return res, nil
}

func (c *Client) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	return nil, nil
}

func (c *Client) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	nsList, err := c.Client.NamespaceList()
	if err != nil {
		return nil, err
	}
	var res []*client.Namespace
	for _, o := range nsList {
		res = append(res, &client.Namespace{
			Name:        o.Name,
			Path:        o.Path,
			Kind:        o.Kind,
			ProjectUUID: o.ProjectUUID,
		})
	}
	return res, nil
}

func (c *Client) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	projects, err := c.Client.RepoList(opt.Namespace, opt.Key, opt.PerPage)
	if err != nil {
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}
	var res []*client.Project
	for _, project := range projects {
		res = append(res, &client.Project{
			Name:          project.Name,
			Description:   project.Description,
			DefaultBranch: project.DefaultBranch,
			Namespace:     project.Namespace,
			RepoUUID:      project.RepoUUID,
			RepoID:        project.RepoID,
		})
	}
	return res, nil
}
