package ilyshin

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Project struct {
	ID                int               `json:"id"`
	Description       string            `json:"description"`
	Name              string            `json:"name"`
	NameWithNamespace string            `json:"name_with_namespace"`
	Path              string            `json:"path"`
	PathWithNamespace string            `json:"path_with_namespace"`
	CreatedAt         *time.Time        `json:"created_at,omitempty"`
	Archived          bool              `json:"archived"`
	DefaultBranch     string            `json:"default_branch"`
	Namespace         *ProjectNamespace `json:"namespace"`
}

type ProjectNamespace struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	FullPath string `json:"full_path"`
}

func (c *Client) ListNamespaces(keyword string, log *zap.SugaredLogger) ([]*Project, error) {
	url := fmt.Sprintf("/api/v4/projects")
	qs := map[string]string{
		"order_by":     "name",
		"sort":         "asc",
		"project_type": "group", // group or project
		"per_page":     "100",
		"simple":       "true",
	}
	if keyword != "" && len(keyword) > 2 {
		qs["search"] = keyword
	}
	var err error
	var gps []*Project
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&gps)); err != nil {
		log.Errorf("Failed to list projects, error: %s", err)
		return gps, err
	}

	return gps, nil
}
