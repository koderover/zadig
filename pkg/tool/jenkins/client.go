package jenkins

import (
	"github.com/imroc/req/v3"
)

type Client struct {
	*req.Client
	User  string
	Token string
}

func NewClient(host, user, token string) (client *Client) {
	client = &Client{
		Client: req.C().
			EnableInsecureSkipVerify().
			SetCommonBasicAuth(user, token).SetBaseURL(host),
	}
	return client
}
