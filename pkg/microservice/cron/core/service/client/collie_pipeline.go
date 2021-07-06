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

package client

import (
	"fmt"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *CollieClient) ListColliePipelines(log *zap.SugaredLogger) ([]*service.PipelineResource, error) {
	url := "/api/collie/api/pipelines"

	ciPipelines := make([]*service.PipelineResource, 0)
	_, err := c.Get(url, httpclient.SetResult(&ciPipelines))
	if err != nil {
		log.Errorf("ListColliePipelines from collie failed, err: %+v", err)
		return nil, err
	}

	return ciPipelines, nil
}

func (c *CollieClient) RunColliePipelineTask(args *service.CreateBuildRequest, log *zap.SugaredLogger) error {
	// collie pipeline list接口返回的pipelineName的格式为：项目名/pipelineName, 需要对进行encode
	encodePipelineName := url.QueryEscape(args.PipelineName)
	url := fmt.Sprintf("/api/collie/api/builds/%s/%s/run", encodePipelineName, args.ProductName)

	resp, err := c.Post(url, httpclient.SetBody(args))
	if err != nil {
		return err
	}
	log.Infof("run collie pipeline %s result %s", args.PipelineName, resp.String())

	return nil
}
