package aslan

import (
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type cluster struct {
	Name       string `json:"name"`
	Production bool   `json:"production"`
	Local      bool   `json:"local"`
}

func (c *Client) AddMultiCluster() error {
	timeStamp := time.Now().Format("20060102150405")
	url := "/cluster/clusters"
	req := cluster{
		Name:  fmt.Sprintf("%s-%s", "local", timeStamp),
		Local: true,
	}

	_, err := c.Post(url, httpclient.SetBody(req))
	if err != nil {
		return fmt.Errorf("Failed to add multi cluster, error: %s", err)
	}

	return nil
}

func (c *Client) ListMultiCluster() ([]*cluster, error) {
	url := "/cluster/clusters"

	clusters := make([]*cluster, 0)
	_, err := c.Get(url, httpclient.SetResult(clusters))
	if err != nil {
		return nil, fmt.Errorf("Failed to list multi cluster, error: %s", err)
	}

	return clusters, nil
}
