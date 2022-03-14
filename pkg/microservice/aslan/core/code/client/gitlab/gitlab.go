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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
)

type Config struct {
	ID          int    `json:"id"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
	Namespace   string `json:"namespace"`
	Region      string `json:"region"`
	// the field and tag not consistent because of db field
	AccessKey string `json:"application_id"`
	SecretKey string `json:"client_secret"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	// the field determine whether the proxy is enabled
	EnableProxy bool `json:"enable_proxy"`
}

type Client struct {
	Client *gitlab.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {

	client, err := gitlab.NewClient(c.Address, c.AccessToken, config.ProxyHTTPSAddr(), c.EnableProxy)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(opt.Namespace, opt.ProjectName, opt.Key, &gitlab.ListOptions{
		Page:        opt.Page,
		PerPage:     opt.PerPage,
		NoPaginated: true,
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
	prs, err := c.Client.ListOpenedProjectMergeRequests(opt.Namespace, opt.ProjectName, opt.TargeBr, opt.Key, &gitlab.ListOptions{
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
