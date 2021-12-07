package aslan

import (
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type cluster struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Status setting.K8SClusterStatus `json:"status"`
	Local  bool                     `json:"local"`
}

func (c *Client) AddLocalCluster() error {
	url := "/cluster/clusters"
	req := cluster{
		ID:     setting.LocalClusterID,
		Name:   fmt.Sprintf("%s-%s", "local", time.Now().Format("20060102150405")),
		Local:  true,
		Status: setting.Normal,
	}

	_, err := c.Post(url, httpclient.SetBody(req))
	if err != nil {
		return fmt.Errorf("Failed to add multi cluster, error: %s", err)
	}

	return nil
}

func (c *Client) GetCluster() (*cluster, error) {
	url := "/cluster/clusters/" + setting.LocalClusterID

	var cluster *cluster
	_, err := c.Get(url, httpclient.SetResult(&cluster))
	if err != nil {
		return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
	}

	return cluster, nil
}
