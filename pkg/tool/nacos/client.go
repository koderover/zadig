/*
Copyright 2023 The KodeRover Authors.

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

package nacos

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
)

func GetNacosCli(serverAddr, userName, password string) (config_client.IConfigClient, error) {
	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogLevel:            "error",
		Username:            userName,
		Password:            password,
	}
	host, err := url.Parse(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("parse nacos address error: %v", err)
	}
	if host.Path == "" {
		host.Path = "/nacos"
	}
	var port uint64

	if host.Port() != "" {
		portInt, err := strconv.Atoi(host.Port())
		if err != nil {
			return nil, fmt.Errorf("parse nacos port error: %v", err)
		}
		port = uint64(portInt)
	}

	serverConfigs := []constant.ServerConfig{
		{
			IpAddr:      host.Hostname(),
			Port:        port,
			Scheme:      host.Scheme,
			ContextPath: host.Path,
		},
	}

	configClient, err := clients.CreateConfigClient(map[string]interface{}{
		"clientConfig":  clientConfig,
		"serverConfigs": serverConfigs,
	})
	if err != nil {
		return configClient, fmt.Errorf("create nacos client error: %v", err)
	}
	return configClient, nil
}
