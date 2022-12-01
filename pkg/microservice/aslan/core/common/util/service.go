/*
Copyright 2022 The KodeRover Authors.

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

package util

import "github.com/koderover/zadig/pkg/setting"

func GetServiceDeployStrategy(serviceName string, strategyMap map[string]string) string {
	if strategyMap == nil {
		return setting.ServiceDeployStrategyDeploy
	}
	if value, ok := strategyMap[serviceName]; !ok || value == "" {
		return setting.ServiceDeployStrategyDeploy
	} else {
		return value
	}
}

func ServiceDeployed(serviceName string, strategyMap map[string]string) bool {
	return GetServiceDeployStrategy(serviceName, strategyMap) == setting.ServiceDeployStrategyDeploy
}

func DeployStrategyChanged(serviceName string, strategyMapOld map[string]string, strategyMapNew map[string]string) bool {
	return ServiceDeployed(serviceName, strategyMapOld) != ServiceDeployed(serviceName, strategyMapNew)
}
