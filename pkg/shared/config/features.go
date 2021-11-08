package config


import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type feature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func (c *Client) CheckFeature(featureName string) (bool, error) {
	url := fmt.Sprintf("/api/v1/features/%s",featureName)

	fs := &feature{}
	_, err := c.Get(url, httpclient.SetResult(fs))
	if err != nil {
		return false, err
	}

	return fs.Enabled, nil
}