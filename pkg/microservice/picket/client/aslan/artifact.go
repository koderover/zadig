package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) GetArtifactByImage(header http.Header, qs url.Values, image string) ([]byte, error) {
	url := fmt.Sprintf("/delivery/artifacts/image?image=%s", image)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) GetArtifactInfo(header http.Header, qs url.Values, id string) ([]byte, error) {
	url := fmt.Sprintf("/delivery/artifacts/%s", id)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
