package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListProjects(header http.Header, qs url.Values) ([]byte, error) {
	url := "/project/projects"

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CreateProject(header http.Header, body []byte, projectName string) ([]byte, error) {
	url := fmt.Sprintf("/project/products/%s", projectName)
	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetBody(body))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
