package aslan

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CleanConfig struct {
	Cron        string `json:"cron"`
	CronEnabled bool   `json:"cron_enabled"`
}

func (c *Client) GetDockerCleanConfig() (*CleanConfig, error) {
	url := "/system/cleanCache/state"

	res := new(CleanConfig)

	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) DockerClean() error {
	url := "/system/cleanCache/oneClick"
	_, err := c.Post(url)
	return err
}
