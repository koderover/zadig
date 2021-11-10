package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListTestWorkflows(testName string, header http.Header, qs url.Values) ([]byte, error) {
	url := fmt.Sprintf("/workflow/workflow/testName/%s", testName)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CreateWorkflowTask(header http.Header, qs url.Values, body []byte) ([]byte, error) {
	url := "/workflow/workflowtask"

	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetBody(body))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CancelWorkflowTask(header http.Header, qs url.Values, id string, name string) (statusCode int, err error) {
	url := fmt.Sprintf("/workflow/workflowtask/id/%s/pipelines/%s", id, name)

	res, err := c.Delete(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return res.StatusCode(), nil
}
