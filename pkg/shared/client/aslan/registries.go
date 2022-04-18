package aslan

import "github.com/koderover/zadig/pkg/tool/httpclient"

func (c *Client) ListRegistries() ([]*RegistryInfo, error) {
	url := "/system/registry"
	res := make([]*RegistryInfo, 0)

	_, err := c.Get(url, httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}
