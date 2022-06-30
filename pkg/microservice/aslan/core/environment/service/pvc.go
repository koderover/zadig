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
	"sort"
	"sync"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type ListPvcsResponse struct {
	*ResourceResponseBase
	PvcName      string `json:"pvc_name"`
	Status       string `json:"status"`
	Volume       string `json:"volume"`
	AccessModes  string `json:"access_modes"`
	StorageClass string `json:"storageclass"`
	Capacity     string `json:"capacity"`
}

func ListPvcs(envName, productName string, log *zap.SugaredLogger) ([]*ListPvcsResponse, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}

	Pvcs, err := getter.ListPvcs(product.Namespace, nil, kubeClient)
	if err != nil {
		log.Error(err)
		return nil, e.ErrListResources.AddDesc(err.Error())
	}
	envSvcDepends, err := commonrepo.NewEnvSvcDependColl().List(&commonrepo.ListEnvSvcDependOption{ProductName: productName, EnvName: envName})
	if err != nil && commonrepo.IsErrNoDocuments(err) {
		return nil, e.ErrListResources.AddDesc(err.Error())
	}

	var res []*ListPvcsResponse
	var mutex sync.Mutex
	var wg sync.WaitGroup
	pvcProcess := func(pvc *v1.PersistentVolumeClaim) error {
		pvc.SetManagedFields(nil)
		pvc.SetResourceVersion("")
		if pvc.APIVersion == "" {
			pvc.APIVersion = "v1"
		}
		if pvc.Kind == "" {
			pvc.Kind = "PersistentVolumeClaim"
		}
		yamlData, err := yaml.Marshal(pvc)
		if err != nil {
			return err
		}
		accessModes := ""
		lenAm := len(pvc.Spec.AccessModes)
		for i, am := range pvc.Spec.AccessModes {
			accessModes += string(am)
			if i < lenAm-1 {
				accessModes += ","
			}
		}
		var tempSvcs []string
	LB:
		for _, svcDepend := range envSvcDepends {
			for _, pvcc := range svcDepend.Pvcs {
				if pvcc == pvc.Name {
					tempSvcs = append(tempSvcs, svcDepend.ServiceModule)
					continue LB
				}
			}
		}

		storageClassName := ""
		if pvc.Spec.StorageClassName != nil {
			storageClassName = *pvc.Spec.StorageClassName
		}
		capacity := ""
		resourceList := pvc.Status.Capacity[v1.ResourceStorage]
		if &resourceList != nil {
			capacity = resourceList.String()
		}
		resElem := &ListPvcsResponse{
			PvcName:      pvc.Name,
			Status:       string(pvc.Status.Phase),
			Volume:       pvc.Spec.VolumeName,
			AccessModes:  accessModes,
			StorageClass: storageClassName,
			Capacity:     capacity,
			ResourceResponseBase: &ResourceResponseBase{
				Name:        pvc.Name,
				Type:        config.CommonEnvCfgTypePvc,
				EnvName:     envName,
				ProjectName: productName,
				YamlData:    string(yamlData),
				Services:    tempSvcs,
				CreateTime:  pvc.GetCreationTimestamp().Time,
			},
		}
		resElem.setSourceDetailData(pvc)
		mutex.Lock()
		res = append(res, resElem)
		mutex.Unlock()
		return nil
	}
	for _, pvcElem := range Pvcs {
		wg.Add(1)
		go func(pvc *v1.PersistentVolumeClaim) {
			defer wg.Done()
			err := pvcProcess(pvc)
			if err != nil {
				log.Errorf("ListPvcs ns:%s name:%s pvcProcess err:%s", pvc.Namespace, pvc.Name, err)
			}
		}(pvcElem)
	}
	wg.Wait()
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].PvcName < res[j].PvcName
	})
	return res, nil
}

type UpdatePvcArgs struct {
	EnvName              string `json:"env_name"`
	ProductName          string `json:"product_name"`
	PvcName              string `json:"pvc_name"`
	YamlData             string `json:"yaml_data"`
	RestartAssociatedSvc bool   `json:"restart_associated_svc"`
}

func UpdatePvc(args *models.CreateUpdateCommonEnvCfgArgs, userName string, log *zap.SugaredLogger) error {
	js, err := yaml.YAMLToJSON([]byte(args.YamlData))
	pvc := &corev1.PersistentVolumeClaim{}
	err = json.Unmarshal(js, pvc)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	if pvc.Name != args.Name {
		return e.ErrUpdateResource.AddDesc("pvc Yaml Name is incorrect")
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrUpdateResource.AddErr(err)
	}

	yamlData, err := ensureLabelAndNs(pvc, product.Namespace, args.ProductName)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	err = updater.UpdatePvc(product.Namespace, pvc, clientset)
	if err != nil {
		log.Error(err)
		return e.ErrUpdateResource.AddDesc(err.Error())
	}
	envPvc := &models.EnvResource{
		ProductName:    args.ProductName,
		UpdateUserName: userName,
		EnvName:        args.EnvName,
		Namespace:      product.Namespace,
		Name:           pvc.Name,
		YamlData:       yamlData,
		Type:           string(config.CommonEnvCfgTypePvc),
		SourceDetail:   args.SourceDetail,
		AutoSync:       args.AutoSync,
	}
	if commonrepo.NewEnvResourceColl().Create(envPvc) != nil {
		return e.ErrUpdateResource.AddDesc(err.Error())
	}

	if !args.RestartAssociatedSvc {
		return nil
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	if err := restartPod(pvc.Name, args.ProductName, args.EnvName, product.Namespace, config.CommonEnvCfgTypeSecret, clientset, kubeClient); err != nil {
		return e.ErrRestartService.AddDesc(err.Error())
	}
	return nil
}
