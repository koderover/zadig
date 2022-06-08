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
	"time"

	helmclient "github.com/mittwald/go-helm-client"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

const (
	timeout = 60
)

func DeleteProduct(username, envName, productName, requestID string, log *zap.SugaredLogger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddDesc("not found")
	}

	restConfig, err := kube.GetRESTConfig(productInfo.ClusterID)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	// 设置产品状态
	err = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusDeleting)
	if err != nil {
		log.Errorf("[%s][%s] update product status error: %v", username, productInfo.Namespace, err)
		return e.ErrDeleteEnv.AddDesc("更新集成环境状态失败: " + err.Error())
	}

	log.Infof("[%s] delete product %s", username, productInfo.Namespace)
	LogProductStats(username, setting.DeleteProductEvent, productName, requestID, eventStart, log)

	switch productInfo.Source {
	case setting.SourceFromHelm:
		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		go func() {
			var hc helmclient.Client
			var err error
			defer func() {
				if err != nil {
					// 发送删除产品失败消息给用户
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					SendErrorMessage(username, title, requestID, err, log)
					_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					SendMessage(username, title, content, requestID, log)
				}
			}()

			//卸载helm release资源
			if hc, err = helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace); err == nil {
				for _, services := range productInfo.Services {
					for _, service := range services {
						if err = hc.UninstallRelease(&helmclient.ChartSpec{
							ReleaseName: fmt.Sprintf("%s-%s", productInfo.Namespace, service.ServiceName),
							Namespace:   productInfo.Namespace,
							Wait:        true,
							Force:       true,
							Timeout:     timeout * time.Second * 10,
						}); err != nil {
							log.Errorf("UninstallRelease err:%v", err)
						}
					}
				}
			} else {
				log.Errorf("获取helmClient err:%v", err)
			}

			//删除namespace
			s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
			if err1 := DeleteNamespaceIfMatch(productInfo.Namespace, s, productInfo.ClusterID, log); err1 != nil {
				err = e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg + ": " + err1.Error())
				return
			}
		}()
	case setting.SourceFromExternal:
		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		// 删除workload数据
		tempProduct, err := template.NewProductColl().Find(productName)
		if err != nil {
			log.Errorf("project not found error:%s", err)
		}
		if tempProduct.ProductFeature != nil && tempProduct.ProductFeature.CreateEnvType == setting.SourceFromExternal {
			workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(productInfo.ClusterID, productInfo.Namespace)
			if err != nil {
				log.Errorf("workflowStat not found error:%s", err)
			}
			if workloadStat != nil {
				workloadStat.Workloads = FilterWorkloadsByEnv(workloadStat.Workloads, productInfo.EnvName)
				if err := commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat); err != nil {
					log.Errorf("update workloads fail error:%s", err)
				}
			}
			// 获取所有external的服务
			currentEnvServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(productName, envName)
			if err != nil {
				log.Errorf("failed to list external workload, error:%s", err)
			}

			externalEnvServices, err := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName:    productName,
				ExcludeEnvName: envName,
			})
			if err != nil {
				log.Errorf("failed to list external service, error:%s", err)
			}

			externalEnvServiceM := make(map[string]bool)
			for _, externalEnvService := range externalEnvServices {
				externalEnvServiceM[externalEnvService.ServiceName] = true
			}

			deleteServices := sets.NewString()
			for _, currentEnvService := range currentEnvServices {
				if _, isExist := externalEnvServiceM[currentEnvService.ServiceName]; !isExist {
					deleteServices.Insert(currentEnvService.ServiceName)
				}
			}
			err = commonrepo.NewServiceColl().BatchUpdateExternalServicesStatus(productName, "", setting.ProductStatusDeleting, deleteServices.List())
			if err != nil {
				log.Errorf("UpdateStatus external services error:%s", err)
			}
			// delete services_in_external_env data
			if err = commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
				EnvName:     envName,
			}); err != nil {
				log.Errorf("remove services in external env error:%s", err)
			}
		}
	case setting.SourceFromPM:
		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}
	default:
		go func() {
			var err error
			defer func() {
				if err != nil {
					// 发送删除产品失败消息给用户
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					SendErrorMessage(username, title, requestID, err, log)
					_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					SendMessage(username, title, content, requestID, log)
				}
			}()

			err = DeleteNamespacedResource(productInfo.Namespace, labels.Set{setting.ProductLabel: productName}.AsSelector(), productInfo.ClusterID, log)
			if err != nil {
				err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
				return
			}

			err = DeleteClusterResource(labels.Set{setting.ProductLabel: productName, setting.EnvNameLabel: envName}.AsSelector(), productInfo.ClusterID, log)

			if err != nil {
				err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
				return
			}

			s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
			if err1 := DeleteNamespaceIfMatch(productInfo.Namespace, s, productInfo.ClusterID, log); err1 != nil {
				err = e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg + ": " + err1.Error())
				return
			}

			//// 等待10分钟
			//err = WaitPodsToTerminate(productInfo.Namespace, selector, config.ServiceStartTimeout(), kubeClient, log)
			//if err != nil {
			//	log.Errorf("wait port to terminate error: %v", err)
			//	err = e.ErrDeleteEnv.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
			//	return
			//}

			err = commonrepo.NewProductColl().Delete(envName, productName)
			if err != nil {
				log.Errorf("Product.Delete error: %v", err)
			}
		}()
	}
	return nil
}

func FilterWorkloadsByEnv(exist []commonmodels.Workload, env string) []commonmodels.Workload {
	result := make([]commonmodels.Workload, 0)
	for _, v := range exist {
		if v.EnvName != env {
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
