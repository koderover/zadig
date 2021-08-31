package ilyshin

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListGroupProjects(namespace, keyword string, log *zap.SugaredLogger) ([]*Project, error) {
	url := fmt.Sprintf("/api/v4/groups/%s/projects", namespace)
	qs := map[string]string{
		"order_by": "name",
		"sort":     "asc",
		"per_page": "100",
	}
	if keyword != "" && len(keyword) > 2 {
		qs["search"] = keyword
	}
	var err error
	var gps []*Project
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&gps)); err != nil {
		log.Errorf("Failed to list group projects, error: %s", err)
		return gps, err
	}

	return gps, nil
}
