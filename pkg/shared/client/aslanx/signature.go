/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aslanx

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Signature struct {
	ID        string `json:"id,omitempty"`
	Token     string `json:"token"`
	UpdateBy  string `json:"update_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListSignatures returns all signatures, enabled and err.
// enabled means signature control is enabled.
// If the status code is 503, mark enabled as false.
func (c *Client) ListSignatures(log *zap.SugaredLogger) ([]*Signature, bool, error) {
	url := "/api/enterprise/license"

	signatures := make([]*Signature, 0)
	res, err := c.Get(url, httpclient.SetResult(&signatures))
	if err != nil {
		return nil, true, err
	}
	if res.StatusCode() == http.StatusServiceUnavailable {
		return nil, false, nil
	}

	return signatures, true, nil
}
