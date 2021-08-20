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

package aslan

import (
	"fmt"
	"strconv"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) GetContainerLog(envName, projectName, container, pod string, tails int) (string, error) {
	// 获取当前配置
	url := fmt.Sprintf("/api/aslan/logs/log/pods/%s/containers/%s?",
		pod, container)

	req := map[string]string{
		"productName": projectName,
		"envName":     envName,
		"tails":       strconv.Itoa(tails),
	}

	response, err := c.Get(url, httpclient.SetQueryParams(req))
	if err != nil {
		fmt.Printf("GetEnvsList %s \n", err)
		return "", err
	}

	return string(response.Body()), nil
}
