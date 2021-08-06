package ilyshin

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Branch struct {
	Name               string  `json:"name"`
	Protected          bool    `json:"protected"`
	Merged             bool    `json:"merged"`
	Default            bool    `json:"default"`
	CanPush            bool    `json:"can_push"`
	DevelopersCanPush  bool    `json:"developers_can_push"`
	DevelopersCanMerge bool    `json:"developers_can_merge"`
	Commit             *Commit `json:"commit"`
}

func (c *Client) ListBranches(owner, repo string, log *zap.SugaredLogger) ([]*Branch, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/branches", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "100",
	}

	var err error
	var branches []*Branch
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&branches)); err != nil {
		log.Errorf("Failed to list project branches, error: %s", err)
		return branches, err
	}

	return branches, nil
}

func generateProjectName(owner, repo string) string {
	return fmt.Sprintf("%s/%s", owner, repo)
}
