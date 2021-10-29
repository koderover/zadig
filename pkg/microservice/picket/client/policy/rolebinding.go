package policy

import (
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type RoleBinding struct {
	Name   string `json:"name"`
	UID    string `json:"uid"`
	Role   string `json:"role"`
	Public bool   `json:"public"`
}

func (c *Client) ListRoleBindings(header http.Header, qs url.Values) ([]*RoleBinding, error) {
	url := "/rolebindings"

	res := make([]*RoleBinding, 0)
	_, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}
