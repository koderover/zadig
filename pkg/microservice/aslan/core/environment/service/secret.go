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
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type ListSecretsResponse struct {
	*ResourceResponseBase
	SecretName string `json:"secret_name"`
	SecretType string `json:"secret_type"`
}

func ListSecrets(envName, productName string, log *zap.SugaredLogger) ([]*ListSecretsResponse, error) {
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

	envSvcDepends, err := commonrepo.NewEnvSvcDependColl().List(&commonrepo.ListEnvSvcDependOption{ProductName: productName, EnvName: envName})
	if err != nil && commonrepo.IsErrNoDocuments(err) {
		return nil, e.ErrListResources.AddDesc(err.Error())
	}

	secrets, err := getter.ListSecrets(product.Namespace, kubeClient)
	if err != nil {
		log.Error(err)
		return nil, e.ErrListResources.AddDesc(err.Error())
	}

	var res []*ListSecretsResponse
	var mutex sync.Mutex
	var wg sync.WaitGroup
	secretProcess := func(secret *corev1.Secret) error {
		secret.SetManagedFields(nil)
		secret.SetResourceVersion("")
		if secret.APIVersion == "" {
			secret.APIVersion = "v1"
		}
		if secret.Kind == "" {
			secret.Kind = "Secret"
		}
		yamlData, err := yaml.Marshal(secret)
		if err != nil {
			return err
		}
		var tempSvcs []string
	LB:
		for _, svcDepend := range envSvcDepends {
			for _, secc := range svcDepend.Secrets {
				if secc == secret.Name {
					tempSvcs = append(tempSvcs, svcDepend.ServiceModule)
					continue LB
				}
			}
		}
		resElem := &ListSecretsResponse{
			SecretName: secret.Name,
			SecretType: string(secret.Type),
			ResourceResponseBase: &ResourceResponseBase{
				Name:        secret.Name,
				Type:        config.CommonEnvCfgTypeSecret,
				EnvName:     envName,
				ProjectName: productName,
				YamlData:    string(yamlData),
				Services:    tempSvcs,
				CreateTime:  secret.GetCreationTimestamp().Time,
			},
		}
		resElem.setSourceDetailData(secret)
		mutex.Lock()
		res = append(res, resElem)
		mutex.Unlock()
		return nil
	}

	for _, secret := range secrets {
		wg.Add(1)
		go func(sec *corev1.Secret) {
			defer wg.Done()
			err := secretProcess(sec)
			if err != nil {
				log.Errorf("ListSecrets ns:%s name:%s secretProcess err:%s", sec.Namespace, sec.Name, err)
			}
		}(secret)
	}
	wg.Wait()
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].SecretName < res[j].SecretName
	})
	return res, nil
}

func UpdateSecret(args *models.CreateUpdateCommonEnvCfgArgs, userName string, log *zap.SugaredLogger) error {
	js, err := yaml.YAMLToJSON([]byte(args.YamlData))
	secret := &corev1.Secret{}
	err = json.Unmarshal(js, secret)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	if secret.Name != args.Name {
		return e.ErrUpdateResource.AddDesc("secret Yaml Name is incorrect")
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

	yamlData, err := ensureLabelAndNs(secret, product.Namespace, args.ProductName)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	err = updater.UpdateOrCreateSecret(secret, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrUpdateResource.AddDesc(err.Error())
	}
	envSecret := &models.EnvResource{
		ProductName:    args.ProductName,
		UpdateUserName: userName,
		EnvName:        args.EnvName,
		Namespace:      product.Namespace,
		Name:           secret.Name,
		YamlData:       yamlData,
		Type:           string(config.CommonEnvCfgTypeSecret),
		SourceDetail:   args.SourceDetail,
		AutoSync:       args.AutoSync,
	}
	if commonrepo.NewEnvResourceColl().Create(envSecret) != nil {
		return e.ErrUpdateResource.AddDesc(err.Error())
	}

	if !args.RestartAssociatedSvc {
		return nil
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrUpdateConfigMap.AddErr(err)
	}

	if err := restartPod(secret.Name, args.ProductName, args.EnvName, product.Namespace, config.CommonEnvCfgTypeSecret, clientset, kubeClient); err != nil {
		return e.ErrRestartService.AddDesc(err.Error())
	}
	return nil
}
