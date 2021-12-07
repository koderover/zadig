package aslan

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type cluster struct {
	ID     primitive.ObjectID       `json:"id,omitempty"`
	Name   string                   `json:"name"`
	Status setting.K8SClusterStatus `json:"status"`
	Local  bool                     `json:"local"`
}

func (c *Client) AddLocalCluster() error {
	oid, _ := primitive.ObjectIDFromHex(setting.LocalClusterID)
	url := "/cluster/clusters"
	req := cluster{
		ID:     oid,
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

type clusterResp struct {
	Name  string `json:"name"`
	Local bool   `json:"local"`
}

func (c *Client) GetCluster() (*clusterResp, error) {
	url := fmt.Sprintf("/cluster/clusters/%s", setting.LocalClusterID)

	clusterResp := &clusterResp{}
	_, err := c.Get(url, httpclient.SetResult(clusterResp))
	if err != nil {
		return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
	}

	return clusterResp, nil
}
