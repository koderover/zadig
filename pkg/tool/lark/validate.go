/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lark

import (
	"fmt"
	"net/http"

	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
)

// Validate check whether the lark app id and secret are correct
func Validate(appID, secret string) error {
	resp, err := req.C().R().SetBodyJsonMarshal(map[string]string{
		"app_id":     appID,
		"app_secret": secret,
	}).Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.GetStatusCode())
	}

	msg := gjson.Get(resp.String(), "msg").String()
	if msg != "success" {
		return fmt.Errorf("unexpected response message: %s", msg)
	}
	return nil
}
