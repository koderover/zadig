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

package policy

import (
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/types"
)

type RoleBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Role   string               `json:"role"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

type PolicyBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Policy string               `json:"policy"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

func (c *Client) ListRoleBindings(header http.Header, qs url.Values) ([]*RoleBinding, error) {
	url := "/rolebindings"

	res := make([]*RoleBinding, 0)
	_, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) ListPolicyBindings(header http.Header, qs url.Values) ([]*PolicyBinding, error) {
	url := "/policybindings"

	res := make([]*PolicyBinding, 0)
	_, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) DeleteRoleBindings(userID string, header http.Header, qs url.Values) ([]byte, error) {
	url := "rolebindings/bulk-delete"

	qs.Add("userID", userID)
	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetBody([]byte("{}")))
	if err != nil {
		return []byte{}, err
	}
	return res.Body(), err
}

type SearchSystemRoleBindingArgs struct {
	Uids []string `json:"uids"`
}

func (c *Client) SearchSystemRoleBindings(uids []string) (map[string][]*types.RoleBinding, error) {
	url := "system-rolebindings/search"
	args := SearchSystemRoleBindingArgs{
		Uids: uids,
	}
	result := map[string][]*types.RoleBinding{}
	_, err := c.Post(url, httpclient.SetBody(args), httpclient.SetResult(&result))
	if err != nil {
		return nil, err
	}
	return result, nil
}
