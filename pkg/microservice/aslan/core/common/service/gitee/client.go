package gitee

import "github.com/koderover/zadig/pkg/tool/gitee"

type Client struct {
	*gitee.Client
}

func NewClient(id int, accessToken, proxyAddress string, enableProxy bool) *Client {
	client, _ := gitee.NewClient(id, accessToken, proxyAddress, enableProxy)
	return &Client{
		Client: client,
	}
}
