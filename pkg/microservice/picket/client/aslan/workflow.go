package aslan

import (
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListTestProjects(testName string, header http.Header, qs url.Values) ([]byte, error) {
	url := "/workflow/workflow/testName/" + testName

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
