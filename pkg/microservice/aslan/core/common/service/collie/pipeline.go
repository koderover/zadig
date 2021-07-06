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

package collie

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CiPipelineResource struct {
	Version  string           `json:"version"`
	Kind     string           `json:"kind"`
	Metadata PipelineMetadata `json:"metadata"`
}

type PipelineMetadata struct {
	Name             string `json:"name"`
	Project          string `json:"project"`
	ProjectID        string `json:"projectId"`
	Revision         int    `json:"revision"`
	FilePath         string `json:"filePath"`
	Source           string `json:"source"`
	OriginYamlString string `json:"originYamlString"`
	ID               string `json:"id"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func (c *Client) ListCIPipelines(productName string, log *zap.SugaredLogger) ([]*CiPipelineResource, error) {
	url := "/api/collie/api/pipelines"

	ciPipelines := make([]*CiPipelineResource, 0)
	_, err := c.Get(url, httpclient.SetResult(&ciPipelines), httpclient.SetQueryParam("project", productName))
	if err != nil {
		log.Errorf("ListCIPipelines from collie failed, productName:%s, err:%+v", productName, err)
		return nil, err
	}

	return ciPipelines, nil
}

func (c *Client) DeleteCIPipelines(productName string, log *zap.SugaredLogger) error {
	url := "/api/collie/api/pipelines/" + productName

	_, err := c.Delete(url)
	if err != nil {
		log.Errorf("call collie delete pipeline err:%+v", err)
		return err
	}

	return nil
}
