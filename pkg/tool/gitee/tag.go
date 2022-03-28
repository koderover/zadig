package gitee

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Tag struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Commit  Commit `json:"commit"`
}

type Commit struct {
	Sha  string    `json:"sha"`
	Date time.Time `json:"date"`
}

func (c *Client) ListTags(ctx context.Context, accessToken, owner string, repo string) ([]Tag, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(GiteeHOSTURL),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/tags", owner, repo)
	var tags []Tag
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&tags))
	if err != nil {
		return nil, err
	}
	return tags, nil
}
