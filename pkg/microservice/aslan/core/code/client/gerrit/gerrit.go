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

package gerrit

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	gerrittool "github.com/koderover/zadig/pkg/tool/gerrit"

	"go.uber.org/zap"
)

type Config struct {
	Address     string `json:"address"`
	AccessToken string `json:"access_token"`
	AccessKey   string `json:"application_id"`
	EnableProxy bool   `json:"enable_proxy"`
}

type Client struct {
	Client *gerrit.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	gerritClient := gerrit.NewClient(c.Address, c.AccessToken, config.ProxyHTTPSAddr(), c.EnableProxy)
	return &Client{Client: gerritClient}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(opt.ProjectName)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name: o,
		})
	}
	return res, nil
}

func (c *Client) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	result, err := c.Client.ListTags(opt.ProjectName)
	if err != nil {
		return nil, err
	}
	var res []*client.Tag
	for _, o := range result {
		res = append(res, &client.Tag{
			Name:    o.Ref,
			Message: o.Message,
		})
	}
	return res, nil
}

func (c *Client) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	return nil, nil
}

func (c *Client) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	return []*client.Namespace{{
		Name: gerrit.DefaultNamespace,
		Path: gerrit.DefaultNamespace,
		Kind: client.OrgKind,
	}}, nil
}

func (c *Client) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	projects, err := c.Client.ListProjectsByKey(opt.Key)
	if err != nil {
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}
	var res []*client.Project
	for ind, o := range projects {
		res = append(res, &client.Project{
			ID:            ind,                       // fake id
			Name:          gerrittool.Unescape(o.ID), // id could have %2F
			Description:   o.Description,
			DefaultBranch: "master",
			Namespace:     gerrittool.DefaultNamespace,
		})
	}
	return res, nil
}
