/*
Copyright 2022 The KodeRover Authors.

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

package aslan

import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type GetAesKeyFromEncryptedKeyResp struct {
	PlainText string `json:"plain_text"`
}

func (c *Client) GetTextFromEncryptedKey(encryptedKey string) (*GetAesKeyFromEncryptedKeyResp, error) {
	url := fmt.Sprintf("/system/rsaKey/decryptedText?encryptedKey=%s", encryptedKey)

	res := &GetAesKeyFromEncryptedKeyResp{}
	_, err := c.Get(url, httpclient.SetQueryParam("encryptedKey", encryptedKey), httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, nil
}
