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

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/util"
)

const SplitSymbol = "&"

func FilterWorkloadsByEnv(exist []commonmodels.Workload, productName, env string) []commonmodels.Workload {
	result := make([]commonmodels.Workload, 0)
	for _, v := range exist {
		if v.EnvName != env || v.ProductName != productName {
			result = append(result, v)
		}
	}
	return result
}

func DeleteClusterResource(selector labels.Selector, clusterID string, log *zap.SugaredLogger) error {
	log.Infof("Deleting cluster resources with selector: [%s]", selector)

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", clusterID, err)
		return err
	}

	errors := new(multierror.Error)
	if err := updater.DeleteClusterRoles(selector, clientset); err != nil {
		log.Errorf("failed to delete clusterRoles for clusterID: %s, the error is: %s", clusterID, err)
		errors = multierror.Append(errors, err)
	}

	if err := updater.DeletePersistentVolumes(selector, clientset); err != nil {
		log.Errorf("failed to delete PV for clusterID: %s, the error is: %s", clusterID, err)
		errors = multierror.Append(errors, err)
	}

	return errors.ErrorOrNil()
}

// DeleteNamespacedResource deletes the namespaced resources by labels.
func DeleteNamespacedResource(namespace string, selector labels.Selector, clusterID string, log *zap.SugaredLogger) error {
	log.Infof("Deleting namespaced resources with selector: [%s] in namespace [%s]", selector, namespace)

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", clusterID, err)
		return err
	}

	errors := new(multierror.Error)

	if err := updater.DeleteDeployments(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteDeployments error: %v", err))
	}

	// could have replicas created by deployment
	if err := updater.DeleteReplicaSets(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteReplicaSets error: %v", err))
	}

	if err := updater.DeleteStatefulSets(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteStatefulSets error: %v", err))
	}

	if err := updater.DeleteJobs(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteJobs error: %v", err))
	}

	if err := updater.DeleteServices(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteServices error: %v", err))
	}

	// TODO: Questionable delete logic, needs further attention
	if err := updater.DeleteIngresses(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteIngresses error: %v", err))
	}

	if err := updater.DeleteSecrets(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteSecrets error: %v", err))
	}

	if err := updater.DeleteConfigMaps(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteConfigMaps error: %v", err))
	}

	if err := updater.DeletePersistentVolumeClaims(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeletePersistentVolumeClaim error: %v", err))
	}

	if err := updater.DeleteServiceAccounts(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteServiceAccounts error: %v", err))
	}

	if err := updater.DeleteCronJobs(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteCronJobs error: %v", err))
	}

	if err := updater.DeleteRoleBindings(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteRoleBinding error: %v", err))
	}

	if err := updater.DeleteRoles(namespace, selector, clientset); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteRole error: %v", err))
	}

	return errors.ErrorOrNil()
}

func DeleteNamespaceIfMatch(namespace string, selector labels.Selector, clusterID string, log *zap.SugaredLogger) error {
	log.Infof("Checking if namespace [%s] has matching labels: [%s]", namespace, selector.String())

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", clusterID, err)
		return err
	}

	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to list namespace to delete matching namespace in cluster ID: %s, the error is: %s", clusterID, err)
		return err
	}

	if selector.Matches(labels.Set(ns.Labels)) {
		return updater.DeleteNamespace(namespace, clientset)
	}

	return nil
}

func GetProductEnvNamespace(envName, productName, namespace string) string {
	if namespace != "" {
		return namespace
	}
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		product = &commonmodels.Product{EnvName: envName, ProductName: productName}
		return product.GetNamespace()
	}
	return product.Namespace
}

func GetProductTargetMap(prod *commonmodels.Product) (map[string][]commonmodels.DeployEnv, map[string]string) {
	resp := make(map[string][]commonmodels.DeployEnv)
	imageNameM := make(map[string]string)
	if prod.Source == setting.SourceFromExternal {
		services, _ := commonrepo.NewServiceColl().ListExternalWorkloadsBy(prod.ProductName, prod.EnvName)

		currentServiceNames := sets.NewString()
		for _, service := range services {
			currentServiceNames.Insert(service.ServiceName)
		}

		servicesInExternalEnv, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
			ProductName: prod.ProductName,
			EnvName:     prod.EnvName,
		})

		externalServiceNames := sets.NewString()
		for _, serviceInExternalEnv := range servicesInExternalEnv {
			if !currentServiceNames.Has(serviceInExternalEnv.ServiceName) {
				externalServiceNames.Insert(serviceInExternalEnv.ServiceName)
			}
		}

		if len(externalServiceNames) > 0 {
			newServices, _ := commonrepo.NewServiceColl().ListExternalWorkloadsBy(prod.ProductName, "", externalServiceNames.List()...)
			for _, service := range newServices {
				services = append(services, service)
			}
		}

		for _, service := range services {
			for _, container := range service.Containers {
				env := service.ServiceName + "/" + container.Name
				deployEnv := commonmodels.DeployEnv{Type: setting.K8SDeployType, Env: env}
				target := strings.Join([]string{service.ProductName, service.ServiceName, container.Name}, SplitSymbol)
				resp[target] = append(resp[target], deployEnv)

				imageNameM[target] = util.GetImageNameFromContainerInfo(container.ImageName, container.Name)
			}
		}
		return resp, imageNameM
	}
	for _, services := range prod.Services {
		for _, serviceObj := range services {
			switch serviceObj.Type {
			case setting.K8SDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.K8SDeployType, Env: env}
					target := strings.Join([]string{serviceObj.ProductName, serviceObj.ServiceName, container.Name}, SplitSymbol)
					resp[target] = append(resp[target], deployEnv)

					imageNameM[target] = util.GetImageNameFromContainerInfo(container.ImageName, container.Name)
				}
			case setting.PMDeployType:
				deployEnv := commonmodels.DeployEnv{Type: setting.PMDeployType, Env: serviceObj.ServiceName}
				target := strings.Join([]string{serviceObj.ProductName, serviceObj.ServiceName, serviceObj.ServiceName}, SplitSymbol)
				resp[target] = append(resp[target], deployEnv)
			case setting.HelmDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.HelmDeployType, Env: env}
					target := strings.Join([]string{serviceObj.ProductName, serviceObj.ServiceName, container.Name}, SplitSymbol)
					resp[target] = append(resp[target], deployEnv)

					imageNameM[target] = util.GetImageNameFromContainerInfo(container.ImageName, container.Name)
				}
			}
		}
	}
	return resp, imageNameM
}
