package opa

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

// Evaluate evaluates the query with the given input and return a json response which has a field called "result"
func (c *Client) Evaluate(query string, input interface{}) ([]byte, error) {
	url := "v1/data"
	req := struct{
		Input interface{} `json:"input"`
	}{
		Input: input,
	}

	queryURL := fmt.Sprintf("%s/%s", url, strings.ReplaceAll(query, ".", "/"))
	res, err := c.Post(queryURL, httpclient.SetBody(req))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
