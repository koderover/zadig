package aslan

import (
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) Overview(header http.Header, qs url.Values) ([]byte, error) {
	url := "/stat/dashboard/overview"
	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
func (c *Client) Test(header http.Header, qs url.Values) ([]byte, error) {
	url := "/stat/dashboard/test"
	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
func (c *Client) Deploy(header http.Header, qs url.Values) ([]byte, error) {
	url := "/stat/dashboard/deploy"
	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
func (c *Client) Build(header http.Header, qs url.Values) ([]byte, error) {
	url := "/stat/dashboard/build"
	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
