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
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListEnvironments(projectName string) ([]ListEnvsResp, error) {

	url := fmt.Sprintf("/api/aslan/environment/environments?productName=%s", projectName)
	resp := make([]ListEnvsResp, 0)
	_, err := c.Get(url, httpclient.SetResult(&resp))
	if err != nil {
		fmt.Printf("GetEnvsList %s \n", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) GetAllServicesByEnv(envName, projectName string) ([]ServiceStatus, error) {
	listEnv, err := c.ListEnvironments(projectName)
	if err != nil {
		fmt.Printf("GetEnvsList error in GetAllProjectByEnv %s \n", err)
		return nil, err
	}
	serviceNameList := make([]ServiceStatus, 0)
	for i := 0; i < len(listEnv); i++ {
		if listEnv[i].EnvName == envName {
			if listEnv[i].Source == "helm" {

				resp, err := c.GetHelmServices(envName, projectName)
				if err != nil {
					fmt.Printf("c.GetHelmServices error:%s \n", err)
				}
				for _, service := range resp.Services {
					serviceStatus := ServiceStatus{}
					serviceStatus.ServiceName = service.ServiceName
					serviceStatus.Status = service.Status
					serviceNameList = append(serviceNameList, serviceStatus)
				}

			} else if listEnv[i].Source != "helm" {

				serviceDetailList, err := c.GetServices(envName, projectName)
				if err != nil {
					fmt.Printf("c.GetServices error:%s \n", err)
				}
				for _, service := range serviceDetailList {
					serviceStatus := ServiceStatus{}
					serviceStatus.ServiceName = service.ServiceName
					serviceStatus.Status = service.Status
					serviceNameList = append(serviceNameList, serviceStatus)
				}
			}
			if err != nil {
				fmt.Printf("GetEnvsList %s \n", err)
				return nil, err
			}
		}
	}

	return serviceNameList, nil
}

func (c *Client) GetServicesDetail(envName, projectName string) ([]ServiceStatusListResp, error) {

	serviceList, err := c.GetAllServicesByEnv(envName, projectName)
	if err != nil {
		fmt.Printf("getAllProjectByEnv %s \n", err)
		return nil, err
	}

	wg := sync.WaitGroup{}
	serviceStatusList := make([]ServiceStatusListResp, 0)
	for _, service := range serviceList {
		sName := service.ServiceName
		status := service.Status
		wg.Add(1)

		go func(name string) {
			defer func() {
				wg.Done()
			}()

			resp, err := c.GetServicePodDetails(projectName, sName, envName, "")
			if err != nil {
				fmt.Printf("get service detail error:%s,%s \n", err, sName)
			}

			servicesStatus := ServiceStatusListResp{}
			servicesStatus.ServiceName = sName
			servicesStatus.Status = status
			podName := ""
			containerName := ""
			if len(resp.Scales) == 0 {
				fmt.Println("no project scales")
				return

			} else {
				if len(resp.Scales[0].Pods) == 0 {
					fmt.Println("no project pods")
					return
				}
				podNameList := make([]string, 0)
				containerNameList := make([]string, 0)
				for _, pod := range resp.Scales[0].Pods {
					podNameList = append(podNameList, pod.Name)
				}
				podName = strings.Join(podNameList, ",")
				if len(podName) == 0 {
					fmt.Println("podName  error")
				}

				for _, con := range resp.Scales[0].Pods[0].Containers {
					containerNameList = append(containerNameList, con.Name)
				}
				containerName = strings.Join(containerNameList, ",")
				if len(containerName) == 0 {
					fmt.Println("containerName  error")
				}

				servicesStatus.Pod = podName
				servicesStatus.Container = containerName
				serviceStatusList = append(serviceStatusList, servicesStatus)
			}

		}(sName)

	}

	wg.Wait()

	return serviceStatusList, nil
}

func (c *Client) GetHelmServices(envName, projectName string) (ServicesListResp, error) {

	var resp ServicesListResp
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups/helm", projectName)
	_, err := c.Get(url, httpclient.SetQueryParam("envName", envName), httpclient.SetResult(&resp))

	return resp, err
}

func (c *Client) GetServices(envName, projectName string) ([]ServiceDetail, error) {

	ServiceDetailList := make([]ServiceDetail, 0)
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups", projectName)
	_, err := c.Get(url, httpclient.SetQueryParam("envName", envName), httpclient.SetResult(&ServiceDetailList))

	return ServiceDetailList, err
}

func (c *Client) GetServicePodDetails(projectName, sName, envName, envType string) (EnvProjectDetail, error) {

	var resp EnvProjectDetail
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/services/%s",
		projectName, sName)

	req := map[string]string{
		"envName": envName,
		"envType": envType,
	}
	_, err := c.Get(url, httpclient.SetQueryParams(req), httpclient.SetResult(&resp))
	return resp, err
}
