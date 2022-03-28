package gitee

import (
	"context"
	"fmt"

	"gitee.com/openeuler/go-gitee/gitee"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListTags(ctx context.Context, accessToken, owner string, repo string) ([]gitee.Tag, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(GiteeHOSTURL),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/tags", owner, repo)
	var tags []gitee.Tag
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&tags))
	if err != nil {
		return nil, err
	}
	return tags, nil
}
