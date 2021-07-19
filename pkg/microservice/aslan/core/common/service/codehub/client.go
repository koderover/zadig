package codehub

import (
	"github.com/koderover/zadig/pkg/tool/codehub"
)

type Client struct {
	*codehub.CodeHubClient
}

func NewClient(ak, sk, region string) *Client {
	c := codehub.NewCodeHubClient(ak, sk, region)
	return &Client{
		CodeHubClient: c,
	}
}
