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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

func (c *CollieClient) ListColliePipelines(log *xlog.Logger) ([]*service.PipelineResource, error) {
	collieApiAddress := c.ApiAddress
	if collieApiAddress == "" {
		return nil, errors.New("collieApiAddress cannot be empty")
	}

	header := http.Header{}
	header.Add("authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.ApiRootKey))

	url := collieApiAddress + "/api/collie/api/pipelines"

	resp, err := util.SendRequest(url, http.MethodGet, header, nil)
	if err != nil {
		return nil, err
	}

	pipelineList := make([]*service.PipelineResource, 0)
	err = json.Unmarshal(resp, &pipelineList)
	if err != nil {
		return nil, err
	}
	return pipelineList, nil
}

func (c *CollieClient) RunColliePipelineTask(args *service.CreateBuildRequest, log *xlog.Logger) error {
	collieApiAddress := c.ApiAddress
	if collieApiAddress == "" {
		return errors.New("collieApiAddress cannot be empty")
	}

	// collie pipeline list接口返回的pipelineName的格式为：项目名/pipelineName, 需要对进行encode
	encodePipelineName := url.QueryEscape(args.PipelineName)
	uri := fmt.Sprintf("/api/collie/api/builds/%s/%s/run", encodePipelineName, args.ProductName)
	url := collieApiAddress + uri

	header := http.Header{}
	header.Add("authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.ApiRootKey))

	body, _ := json.Marshal(args)

	resp, err := util.SendRequest(url, http.MethodPost, header, body)
	if err != nil {
		return err
	}

	log.Infof("run collie pipeline %s result %s", args.PipelineName, string(resp))
	return nil
}
