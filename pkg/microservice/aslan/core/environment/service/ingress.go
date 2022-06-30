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

package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type ListIngressesResponse struct {
	*ResourceResponseBase
	IngressName string `json:"ingress_name"`
	HostInfo    string `json:"host_info"`
	Address     string `json:"address"`
	Ports       string `json:"ports"`
	ErrorReason string `json:"error_reason"`
}

func ListIngresses(envName, productName string, log *zap.SugaredLogger) ([]*ListIngressesResponse, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	kubeCli, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	cliSet, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	version, err := cliSet.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", product.ClusterID, err)
		return nil, err
	}
	ingresss, err := getter.ListIngresses(product.Namespace, kubeCli, kubeclient.VersionLessThan122(version))
	if err != nil {
		log.Error(err)
		return nil, e.ErrListResources.AddDesc(err.Error())
	}

	var res []*ListIngressesResponse
	for _, ingress := range ingresss.Items {
		ingress.SetManagedFields(nil)
		ingress.SetResourceVersion("")
		yamlData, err := yaml.Marshal(ingress.Object)
		if err != nil {
			log.Error(err)
			return nil, e.ErrListResources.AddDesc(err.Error())
		}

		if ingress.GetAPIVersion() == v1.SchemeGroupVersion.String() {
			itemJson, err := ingress.MarshalJSON()
			if err != nil {
				return nil, err
			}
			itemIngress := &v1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				return nil, err
			}
			hostInfo := ""
			for _, rule := range itemIngress.Spec.Rules {
				hostInfo += rule.Host + ","
			}
			hostInfo = strings.TrimSuffix(hostInfo, ",")
			address, ports, errorReason := "", "", ""
			for _, addr := range itemIngress.Status.LoadBalancer.Ingress {
				address += addr.IP + ","
				for _, portStatus := range addr.Ports {
					if portStatus.Error != nil {
						errorReason += *portStatus.Error + ";"
					} else {
						ports += string(portStatus.Port) + ","
					}
				}
			}
			address = strings.TrimSuffix(address, ",")
			errorReason = strings.TrimSuffix(errorReason, ";")
			ports = strings.TrimSuffix(ports, ",")

			resElem := &ListIngressesResponse{
				IngressName: ingress.GetName(),
				HostInfo:    hostInfo,
				Ports:       ports,
				ErrorReason: errorReason,
				Address:     address,
				ResourceResponseBase: &ResourceResponseBase{
					Name:        ingress.GetName(),
					Type:        config.CommonEnvCfgTypeIngress,
					EnvName:     envName,
					ProjectName: productName,
					YamlData:    string(yamlData),
					CreateTime:  ingress.GetCreationTimestamp().Time,
				},
			}
			resElem.setSourceDetailData(&ingress)
			res = append(res, resElem)
		}

		if ingress.GetAPIVersion() == extensionsv1beta1.SchemeGroupVersion.String() {
			itemJson, err := ingress.MarshalJSON()
			if err != nil {
				return nil, err
			}
			itemIngress := &extensionsv1beta1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				return nil, err
			}
			hostInfo, ports := "", ""
			for _, rule := range itemIngress.Spec.Rules {
				hostInfo += rule.Host + ","
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						ports += path.Backend.ServicePort.String() + ","
					}
				}
			}
			hostInfo = strings.TrimSuffix(hostInfo, ",")
			address, errorReason := "", ""
			for _, addr := range itemIngress.Status.LoadBalancer.Ingress {
				address += addr.IP + ","
				for _, portStatus := range addr.Ports {
					if portStatus.Error != nil {
						errorReason += *portStatus.Error + ";"
					}
				}
			}
			address = strings.TrimSuffix(address, ",")
			errorReason = strings.TrimSuffix(errorReason, ";")
			ports = strings.TrimSuffix(ports, ",")

			resElem := &ListIngressesResponse{
				IngressName: ingress.GetName(),
				HostInfo:    hostInfo,
				Ports:       ports,
				ErrorReason: errorReason,
				Address:     address,
				ResourceResponseBase: &ResourceResponseBase{
					Name:        ingress.GetName(),
					Type:        config.CommonEnvCfgTypeIngress,
					EnvName:     envName,
					ProjectName: productName,
					YamlData:    string(yamlData),
					CreateTime:  ingress.GetCreationTimestamp().Time,
				},
			}
			resElem.setSourceDetailData(&ingress)
			res = append(res, resElem)
		}
	}
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].IngressName < res[j].IngressName
	})
	return res, nil
}

type UpdateIngressArgs struct {
	EnvName              string `json:"env_name"`
	ProductName          string `json:"product_name"`
	IngressName          string `json:"ingress_name"`
	YamlData             string `json:"yaml_data"`
	RestartAssociatedSvc bool   `json:"restart_associated_svc"`
}

func UpdateOrCreateIngress(args *models.CreateUpdateCommonEnvCfgArgs, userName string, isCreate bool, log *zap.SugaredLogger) error {
	u, err := serializer.NewDecoder().YamlToUnstructured([]byte(args.YamlData))
	if err != nil {
		log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", args.YamlData, err)
		return e.ErrUpdateResource.AddDesc("ingress Yaml Name is incorrect")
	}
	if !isCreate && u.GetName() != args.Name {
		return e.ErrUpdateResource.AddDesc("ingress Yaml Name is incorrect")
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	yamlData, err := ensureLabelAndNs(u, product.Namespace, args.ProductName)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	err = updater.UpdateOrCreateUnstructured(u, kubeClient)
	if err != nil {
		log.Errorf("Failed to UpdateOrCreateIngress %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
		return e.ErrUpdateResource.AddErr(fmt.Errorf("Failed to UpdateOrCreateIngress %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err))
	}
	envIngress := &models.EnvResource{
		ProductName:    args.ProductName,
		UpdateUserName: userName,
		EnvName:        args.EnvName,
		Namespace:      product.Namespace,
		Name:           u.GetName(),
		YamlData:       yamlData,
		Type:           string(config.CommonEnvCfgTypeIngress),
		SourceDetail:   args.SourceDetail,
		AutoSync:       args.AutoSync,
	}
	if commonrepo.NewEnvResourceColl().Create(envIngress) != nil {
		return e.ErrUpdateResource.AddDesc(err.Error())
	}
	return err
}
