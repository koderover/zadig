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
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) GetEnvsList(projectName string) ([]*ListEnvsResp, error) {

	url := fmt.Sprintf("/api/aslan/environment/environments?productName=%s", projectName)
	resp := make([]*ListEnvsResp, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp))
	if err != nil {
		fmt.Errorf("GetEnvsList %s", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) GetAllProjectByEnv(envName, projectName string) ([]string, error) {
	listEnv, err := c.GetEnvsList(projectName)
	if err != nil {
		fmt.Errorf("GetEnvsList error in GetAllProjectByEnv %s", err)
		return nil, err
	}
	serviceNameList := make([]string, 0)
	for i := 0; i < len(listEnv); i++ {
		if listEnv[i].EnvName == envName {
			if listEnv[i].Source == "helm" {
				var resp ServicesListResp
				url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups/helm?envName=%s", projectName, envName)
				_, err = c.Get(url, httpclient.SetResult(&resp))
				for _, service := range resp.Services {
					serviceNameList = append(serviceNameList, service.ServiceName)
				}

			} else if listEnv[i].Source != "helm" {
				serviceDetailList := make([]ServiceDetail, 0)
				url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups?envName=%s", projectName, envName)
				_, err = c.Get(url, httpclient.SetResult(&serviceDetailList))
				for _, service := range serviceDetailList {
					serviceNameList = append(serviceNameList, service.ServiceName)
				}
			}
			if err != nil {
				fmt.Printf("GetEnvsList %s", err)
				return nil, err
			}
		}
	}

	return serviceNameList, nil
}

func (c *Client) GetServicesDetail(envName, projectName string) ([]*ServiceStatusListResp, error) {

	serviceNameList, err := c.GetAllProjectByEnv(envName, projectName)
	if err != nil {
		fmt.Errorf("getAllProjectByEnv %s", err)
		return nil, err
	}

	wg := sync.WaitGroup{}
	serviceStatusList := make([]*ServiceStatusListResp, 0)
	for _, serviceName := range serviceNameList {
		sName := serviceName
		wg.Add(1)
		var resp EnvProjectDetail
		go func(name string) {
			defer func() {
				wg.Done()
			}()

			url := fmt.Sprintf("/api/aslan/environment/environments/%s/services/%s?envName=%s&envType=",
				projectName, sName, envName)
			_, err = c.Get(url, httpclient.SetResult(&resp))
			if err != nil {
				fmt.Errorf("get service detail error:%s,%s \n", err, sName)
			}

			servicesStatus := ServiceStatusListResp{}
			servicesStatus.ServiceName = resp.ServiceName
			podName := ""
			status := "Running"
			if len(resp.Scales) <= 0 {
				fmt.Errorf("no project scales")
				return

			} else {
				if len(resp.Scales[0].Pods) <= 0 {
					fmt.Errorf("no project pods")
					return
				}
				if len(resp.Scales[0].Pods[0].Containers) <= 0 {
					fmt.Errorf("no project containers")
					return
				}

				for _, pod := range resp.Scales[0].Pods {
					podName = fmt.Sprintf("%s%s%s", podName,
						pod.Name, ",")
					if pod.Status != status {
						status = pod.Status
					}
				}
				podName = strings.Trim(podName, ",")
				if len(podName) <= 0 {
					fmt.Errorf("pods  error:\n")
				}

				containers := ""
				for _, con := range resp.Scales[0].Pods[0].Containers {
					containers = fmt.Sprintf("%s%s%s", containers,
						con.Name, ",")
				}
				containers = strings.Trim(containers, ",")
				if len(containers) <= 0 {
					fmt.Errorf("containers error:\n")
				}

				servicesStatus.Pod = podName
				servicesStatus.Status = status
				servicesStatus.Container = containers
				serviceStatusList = append(serviceStatusList, &servicesStatus)
			}

		}(sName)

	}

	wg.Wait()

	return serviceStatusList, nil
}

func (c *Client) getLogsByHttp(envName, projectName, container, pod string, tails int) (string, error) {
	// 获取当前配置
	url := fmt.Sprintf("/api/aslan/logs/log/pods/%s/containers/%s?tails=%s&envName=%s&productName=%s",
		pod, container, strconv.Itoa(tails), envName, projectName)

	response, err := c.Get(url)
	if err != nil {
		fmt.Errorf("GetEnvsList %s", err)
		return "", err
	}

	return string(response.Body()), nil
}
