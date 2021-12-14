package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListDelivery(header http.Header, qs url.Values, productName, workflowName, taskId, perPage, page string) ([]byte, error) {
	url := fmt.Sprintf("/delivery/releases?orgId=1&productName=%s&workflowName=%s&taskId=%s&per_page=%s&page=%s", productName, workflowName, taskId, perPage, page)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
