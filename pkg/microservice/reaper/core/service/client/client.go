package client

import (
	"net/http"
	"net/http/cookiejar"
)

type Client struct {
	APIBase string
	Token   string
	Conn    *http.Client
}

// NewAslanClient is to get aslan client func
func NewAslanClient(host, token string) *Client {
	jar, _ := cookiejar.New(nil)

	c := &Client{
		Token:   token,
		APIBase: host,
		Conn:    &http.Client{Transport: http.DefaultTransport, Jar: jar},
	}

	return c
}
