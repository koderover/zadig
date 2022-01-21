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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Label struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"               json:"id,omitempty"`
	Type       string             `bson:"type"                        json:"type"`
	Key        string             `bson:"key"                         json:"key"`
	Value      string             `bson:"value"                       json:"value"`
	CreateBy   string             `bson:"create_by"                   json:"create_by"`
	CreateTime int64              `bson:"create_time"                 json:"create_time"`
}

type ListResourcesByLabelsReq struct {
	LabelFilters []Label `json:"label_filters"`
}

type Resource struct {
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
	Type        string `json:"type"`
}

type ListResourcesByLabelsResp struct {
	Resources map[string][]Resource `json:"resources"`
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

type LabelModel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ListLabelsArgs struct {
	Labels []LabelModel `json:"labels"`
}

type ListLabelsResp struct {
	Labels []Label `json:"labels"`
}

func (c *Client) ListLabels(request ListLabelsArgs) (*ListLabelsResp, error) {
	url := "/label/labels"

	var resources *ListLabelsResp
	_, err := c.Post(url, httpclient.SetBody(request), httpclient.SetResult(&resources))
	if err != nil {
		log.Errorf("Failed to ListLabels, error: %s", err)
		return nil, err
	}

	return resources, nil
}

type ListLabelsByResourcesReq struct {
	Resources []Resource `json:"resources"`
}

type ListLabelsByResourcesResp struct {
	Labels map[string][]*Label `json:"labels"`
}

type CreateLabelsArgs struct {
	Labels []Label `json:"labels"`
}

func (c *Client) ListLabelsByResources(request ListLabelsByResourcesReq) (*ListLabelsByResourcesResp, error) {
	url := "/label/labels/labels-by-resources"

	var resources *ListLabelsByResourcesResp
	_, err := c.Post(url, httpclient.SetBody(request), httpclient.SetResult(&resources))
	if err != nil {
		log.Errorf("Failed to ListLabelsByResources, error: %s", err)
		return nil, err
	}

	return resources, nil
}

func (c *Client) CreateLabels(request CreateLabelsArgs) (*CreateLabelsResp, error) {
	url := "/label/labels"
	var resources *CreateLabelsResp
	_, err := c.Post(url, httpclient.SetBody(request), httpclient.SetResult(&resources))
	if err != nil {
		log.Errorf("Failed to CreateLabels, error: %s", err)
		return nil, err
	}
	return resources, nil
}

func (c *Client) DeleteLabels(ids []string) error {
	url := "/label/labels?force=true"
	_, err := c.Delete(url, httpclient.SetBody(ids))
	if err != nil {
		log.Errorf("Failed to CreateLabels, error: %s", err)
		return err
	}

	return nil
}
