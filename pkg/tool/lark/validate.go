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
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

// Validate check whether the lark app id and secret are correct
func Validate(appID, secret string, larkType string) error {
	baseUrl := GetLarkBaseUrl(larkType)
	resp, err := req.C().R().SetBodyJsonMarshal(map[string]string{
		"app_id":     appID,
		"app_secret": secret,
	}).Post(baseUrl + "/open-apis/auth/v3/tenant_access_token/internal")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.GetStatusCode())
	}

	code := gjson.Get(resp.String(), "code")
	if !code.Exists() {
		return errors.New("invalid lark response")
	}
	if code.Int() != 0 {
		return fmt.Errorf("unexpected lark response code: %d", code.Int())
	}
	return nil
}
