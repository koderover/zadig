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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type ListResourcesByLabelsReq struct {
	LabelFilters []mongodb.Label `json:"label_filters"`
}

type ListResourcesByLabelsResp struct {
	Resources map[string][]mongodb.Resource `json:"resources"`
}

func (c *Client) ListResourcesByLabels(request ListResourcesByLabelsReq) (*ListResourcesByLabelsResp, error) {
	url := "/label/labels/resources-by-labels"

	var resources *ListResourcesByLabelsResp
	_, err := c.Post(url, httpclient.SetBody(request), httpclient.SetResult(&resources))
	if err != nil {
		log.Errorf("Failed to add audit log, error: %s", err)
		return nil, err
	}

	return resources, nil
}

type ListLabelsByResourcesReq struct {
	Resources []mongodb.Resource `json:"resources"`
}

type ListLabelsByResourcesResp struct {
	Labels map[string][]*models.Label `json:"labels"`
}

func (c *Client) ListLabelsByResources(request ListLabelsByResourcesReq) (*ListLabelsByResourcesResp, error) {
	url := "/label/labels/labels-by-resources"

	var resources *ListLabelsByResourcesResp
	_, err := c.Post(url, httpclient.SetBody(request), httpclient.SetResult(&resources))
	if err != nil {
		log.Errorf("Failed to add audit log, error: %s", err)
		return nil, err
	}

	return resources, nil
}
