package aslan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/microservice/picket/core/public/service"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) GetArtifactByImage(header http.Header, qs url.Values, image string) ([]byte, error) {
	url := fmt.Sprintf("/api/aslan/delivery/artifacts/image/%s", image)

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) GetArtifactInfo(header http.Header, qs url.Values, id string) (*service.DeliveryArtifactInfo, error) {
	url := fmt.Sprintf("/api/aslan/delivery/artifacts/%s", id)

	resp := &service.DeliveryArtifactInfo{}
	_, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}

	return resp, nil
}
