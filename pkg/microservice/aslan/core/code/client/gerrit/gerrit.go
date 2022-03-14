package gerrit

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/tool/gerrit"
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
	Client *gerrit.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	gerritClient := gerrit.NewClient(c.Address, c.AccessToken, config.ProxyHTTPSAddr(), c.EnableProxy)
	return &Client{Client: gerritClient}, nil
}

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(projectName)
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

func (c *Client) ListTags(namespace, projectName, key string, page, perPage int) ([]*client.Tag, error) {
	result, err := c.Client.ListTags(projectName)
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

func (c *Client) ListPrs(namespace, projectName, key, targeBr string, page, perPage int) ([]*client.PullRequest, error) {
	return nil, nil
}
