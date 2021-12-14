package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListDelivery(header http.Header, qs url.Values, productName, workflowName, taskId, perPage, page string) ([]byte, error) {
	url := fmt.Sprintf("/delivery/releases")
	queryParams := make(map[string]string)
	queryParams["productName"] = productName
	queryParams["workflowName"] = workflowName
	queryParams["taskId"] = taskId
	queryParams["per_page"] = perPage
	queryParams["page"] = page
	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetQueryParams(queryParams))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
