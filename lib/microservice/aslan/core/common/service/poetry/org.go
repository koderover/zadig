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

package poetry

import (
	"encoding/json"
	"fmt"
)

type Org struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Token   string `json:"orgToken"`
	Website string `json:"website"`
}

func (p *PoetryClient) GetOrg(orgId int) (org *Org, err error) {
	data, err := p.SendRequest(
		fmt.Sprintf("/directory/organization/%d", orgId),
		"GET",
		nil,
		p.GetRootTokenHeader(),
	)

	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(data), &org)
	return
}

func (p *PoetryClient) GetOrgByToken(orgId int) (org *Org, err error) {
	data, err := p.SendRequest(
		fmt.Sprintf("/directory/organization/%d", orgId),
		"GET",
		nil,
		p.GetRootTokenHeader(),
	)

	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(data), &org)
	return
}
