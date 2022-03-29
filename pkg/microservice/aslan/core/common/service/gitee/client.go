package gitee

import "github.com/koderover/zadig/pkg/tool/gitee"

type Client struct {
	*gitee.Client
	AccessToken string
}

func NewClient(id int, accessToken, proxyAddress string, enableProxy bool) *Client {
	client := gitee.NewClient(id, accessToken, proxyAddress, enableProxy)
	return &Client{
		Client:      client,
		AccessToken: accessToken,
	}
}
