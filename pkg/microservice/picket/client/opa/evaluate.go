package opa

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type InputGenerator func() (*Input, error)

// Evaluate evaluates the query with the given input and return a json response which has a field called "result"
func (c *Client) Evaluate(query string, result interface{}, ig InputGenerator) error {
	input, err := ig()
	if err != nil {
		return err
	}

	url := "v1/data"
	req := struct {
		Input interface{} `json:"input"`
	}{
		Input: input,
	}

	queryURL := fmt.Sprintf("%s/%s", url, strings.ReplaceAll(query, ".", "/"))
	_, err = c.Post(queryURL, httpclient.SetBody(req), httpclient.SetResult(result))
	if err != nil {
		return err
	}

	return nil
}
