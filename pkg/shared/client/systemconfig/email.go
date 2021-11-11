package systemconfig

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Email struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	UserName string `json:"username"`
	Password string `json:"password"`
}

func (c *Client) GetEmailHost() (*Email, error) {
	url := "/emails/internal/host/"

	res := &Email{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, err
}
