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
	"github.com/koderover/zadig/pkg/tool/log"
)

func (c *Client) ListEnvironments(projectName string) ([]*ListEnvsResp, error) {

	url := "/api/aslan/environment/environments"
	resp := make([]*ListEnvsResp, 0)
	_, err := c.Get(url, httpclient.SetQueryParam("productName", projectName), httpclient.SetResult(&resp))
	if err != nil {
		log.Errorf("GetEnvsList %s ", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) ListEnvironment(envName, projectName string) (*ListEnvsResp, error) {

	url := fmt.Sprintf("/api/aslan/environment/environments/%s", projectName)
	var resp ListEnvsResp
	_, err := c.Get(url, httpclient.SetQueryParam("envName", envName), httpclient.SetResult(&resp))
	if err != nil {
		log.Errorf("GetEnvsList %s ", err)
		return nil, err
	}

	return &resp, nil
}

func (c *Client) ListServiceNamesByEnvironment(envName, projectName string) ([]*ServiceStatus, error) {
	listEnv, err := c.ListEnvironment(envName, projectName)
	if err != nil {
		log.Errorf("GetEnvsList error in GetAllProjectByEnv %s", err)
		return nil, err
	}
	serviceNameList := make([]*ServiceStatus, 0)

	if listEnv.Source == "helm" {

		resp, err := c.GetHelmServices(envName, projectName)
		if err != nil {
			log.Errorf("c.GetHelmServices error:%s", err)
			return nil, err
		}
		for _, service := range resp.Services {
			serviceStatus := ServiceStatus{}
			serviceStatus.ServiceName = service.ServiceName
			serviceStatus.Status = service.Status
			serviceNameList = append(serviceNameList, &serviceStatus)
		}

	} else if listEnv.Source != "helm" {

		serviceDetailList, err := c.GetServices(envName, projectName)
		if err != nil {
			log.Errorf("c.GetServices error:%s ", err)
			return nil, err
		}
		for _, service := range serviceDetailList {
			serviceStatus := ServiceStatus{}
			serviceStatus.ServiceName = service.ServiceName
			serviceStatus.Status = service.Status
			serviceNameList = append(serviceNameList, &serviceStatus)
		}
	}

	return serviceNameList, nil
}

func (c *Client) GetServicesDetail(envName, projectName string) ([]*ServiceStatusListResp, error) {

	serviceList, err := c.ListServiceNamesByEnvironment(envName, projectName)
	if err != nil {
		log.Errorf("getAllProjectByEnv %s", err)
		return nil, err
	}

	var wg sync.WaitGroup
	serviceStatusList := make([]*ServiceStatusListResp, 0)
	for _, service := range serviceList {
		status := service.Status
		wg.Add(1)

		go func(name string) {
			defer func() {
				wg.Done()
			}()

			resp, err := c.GetServicePodDetails(projectName, name, envName, "")
			if err != nil {
				log.Errorf("get service detail error:%s,%s", err, name)
				return
			}

			servicesStatus := ServiceStatusListResp{}
			servicesStatus.ServiceName = name
			servicesStatus.Status = status
			podName := ""
			containerName := ""
			if len(resp.Scales) == 0 {
				log.Infof("no project scales")
				return

			} else {
				if len(resp.Scales[0].Pods) == 0 {
					log.Infof("no project pods")
					return
				}
				podNameList := make([]string, 0)
				containerNameList := make([]string, 0)
				for _, pod := range resp.Scales[0].Pods {
					podNameList = append(podNameList, pod.Name)
				}
				podName = strings.Join(podNameList, ",")
				if len(podName) == 0 {
					log.Infof("podName  error")
				}

				for _, con := range resp.Scales[0].Pods[0].Containers {
					containerNameList = append(containerNameList, con.Name)
				}
				containerName = strings.Join(containerNameList, ",")
				if len(containerName) == 0 {
					log.Infof("containerName  error")
				}

				servicesStatus.Pod = podName
				servicesStatus.Container = containerName
				serviceStatusList = append(serviceStatusList, &servicesStatus)
			}

		}(service.ServiceName)

	}

	wg.Wait()

	return serviceStatusList, nil
}

func (c *Client) GetHelmServices(envName, projectName string) (*ServicesListResp, error) {

	var resp ServicesListResp
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups/helm", projectName)
	_, err := c.Get(url, httpclient.SetQueryParam("envName", envName), httpclient.SetResult(&resp))

	return &resp, err
}

func (c *Client) GetServices(envName, projectName string) ([]*ServiceDetail, error) {

	ServiceDetailList := make([]*ServiceDetail, 0)
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/groups", projectName)
	_, err := c.Get(url, httpclient.SetQueryParam("envName", envName), httpclient.SetResult(&ServiceDetailList))

	return ServiceDetailList, err
}

func (c *Client) GetServicePodDetails(projectName, sName, envName, envType string) (*EnvProjectDetail, error) {

	var resp EnvProjectDetail
	url := fmt.Sprintf("/api/aslan/environment/environments/%s/services/%s",
		projectName, sName)

	req := map[string]string{
		"envName": envName,
		"envType": envType,
	}
	_, err := c.Get(url, httpclient.SetQueryParams(req), httpclient.SetResult(&resp))
	return &resp, err
}
