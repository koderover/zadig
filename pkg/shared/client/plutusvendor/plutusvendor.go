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

package plutusvendor

import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CheckSignatrueResp struct {
	Code int64 `json:"code"`
}

func (c *Client) CheckSignature(userNum int64) (*CheckSignatrueResp, error) {
	url := fmt.Sprintf("/signature/check?user_num=%d", userNum)
	res := &CheckSignatrueResp{}
	_, err := c.Post(url, httpclient.SetResult(res))
	return res, err
}

type ZadigXLicenseStatus struct {
	Status           string `json:"status"`
	SystemID         string `json:"system_id"`
	UserLimit        int64  `json:"user_limit"`
	UserCount        int64  `json:"user_count"`
	License          string `json:"license"`
	ExpireAt         int64  `json:"expire_at"`
	AvailableVersion string `json:"available_version"`
	CurrentVersion   string `json:"current_version"`
}

func (c *Client) CheckZadigXLicenseStatus() (*ZadigXLicenseStatus, error) {
	url := fmt.Sprintf("/license")
	res := &ZadigXLicenseStatus{}
	_, err := c.Get(url, httpclient.SetResult(res))
	return res, err
}

func (c *Client) Health() error {
	url := "/health"
	_, err := c.Get(url)
	return err
}
