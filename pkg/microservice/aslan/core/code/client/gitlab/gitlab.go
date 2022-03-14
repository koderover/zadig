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

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(namespace, projectName, key, &gitlab.ListOptions{
		Page:        page,
		PerPage:     perPage,
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

func (c *Client) ListTags(namespace, projectName, key string, page, perPage int) ([]*client.Tag, error) {
	tags, err := c.Client.ListTags(namespace, projectName, &gitlab.ListOptions{
		Page:        page,
		PerPage:     perPage,
		NoPaginated: true,
	}, key)
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

func (c *Client) ListPrs(namespace, projectName, key, targeBr string, page, perPage int) ([]*client.PullRequest, error) {
	prs, err := c.Client.ListOpenedProjectMergeRequests(namespace, projectName, targeBr, key, &gitlab.ListOptions{
		Page:        page,
		PerPage:     perPage,
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
