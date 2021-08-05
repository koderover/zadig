package ilyshin

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Tag struct {
	Name    string       `json:"name"`
	Message string       `json:"message"`
	Release *ReleaseNote `json:"release"`
	Commit  *Commit      `json:"commit"`
}

type ReleaseNote struct {
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

func (c *Client) ListTags(owner, repo string, log *zap.SugaredLogger) ([]*Tag, error) {
	url := fmt.Sprintf("/projects/%s/repository/tags", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "100",
	}

	var err error
	var tags []*Tag
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&tags)); err != nil {
		log.Errorf("Failed to list project tags, error: %s", err)
		return tags, err
	}

	return tags, nil
}
