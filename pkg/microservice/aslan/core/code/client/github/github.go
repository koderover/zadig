package github

import (
	"context"

	github2 "github.com/google/go-github/v35/github"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/tool/git/github"
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

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(context.TODO(), namespace, projectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name:      o.GetName(),
			Protected: o.GetProtected(),
		})
	}
	return res, nil
}

func (c *Client) ListTags(namespace, projectName, key string, page, perPage int) ([]*client.Tag, error) {

	tags, err := c.Client.ListTags(context.TODO(), namespace, projectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Tag
	for _, o := range tags {
		res = append(res, &client.Tag{
			Name:       o.GetName(),
			ZipballURL: o.GetZipballURL(),
			TarballURL: o.GetTarballURL(),
		})
	}
	return res, nil
}

func (c *Client) ListPrs(namespace, projectName, key, targeBr string, page, perPage int) ([]*client.PullRequest, error) {
	prs, err := c.Client.ListPullRequests(context.TODO(), namespace, projectName, &github2.PullRequestListOptions{
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
