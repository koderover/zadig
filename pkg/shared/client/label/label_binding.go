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

package label

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CreateLabelsResp struct {
	LabelMap map[string]string `json:"label_map"`
}

type CreateLabelBindingsArgs struct {
	LabelBindings []LabelBinding `json:"label_bindings"`
}

type LabelBinding struct {
	Resource   Resource `json:"resource"`
	LabelID    string   `json:"label_id"`
	CreateBy   string   `json:"create_by"`
	CreateTime int64    `json:"create_time"`
}

type DeleteLabelBindingsArgs struct {
	LabelBindings []LabelBinding `json:"label_bindings"`
}

func (c *Client) CreateLabelBindings(request CreateLabelBindingsArgs) error {
	url := "/label/labelbindings"

	_, err := c.Post(url, httpclient.SetBody(request))
	if err != nil {
		log.Errorf("Failed to CreateLabelBindings, error: %s", err)
		return err
	}

	return nil
}

func (c *Client) DeleteLabelBindings(request DeleteLabelBindingsArgs) error {
	url := "/label/labelbindings"

	_, err := c.Delete(url, httpclient.SetBody(request))
	if err != nil {
		log.Errorf("Failed to DeleteLabelBindings, error: %s", err)
		return err
	}

	return nil
}
