package codehub

import (
	"github.com/koderover/zadig/pkg/tool/codehub"
)

type Client struct {
	*codehub.CodeHubClient
}

func NewClient(ak, sk string) *Client {
	c := codehub.NewCodeHubClient(ak, sk)
	return &Client{
		CodeHubClient: c,
	}
}
