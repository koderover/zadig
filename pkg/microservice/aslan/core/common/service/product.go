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
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	helmclient "github.com/mittwald/go-helm-client"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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
	productInfo, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddDesc("not found")
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	restConfig, err := kube.GetRESTConfig(productInfo.ClusterID)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	// 设置产品状态
	err = mongodb.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusDeleting)
	if err != nil {
		log.Errorf("[%s][%s] update product status error: %v", username, productInfo.Namespace, err)
		return e.ErrDeleteEnv.AddDesc("更新集成环境状态失败: " + err.Error())
	}

	log.Infof("[%s] delete product %s", username, productInfo.Namespace)
	LogProductStats(username, setting.DeleteProductEvent, productName, requestID, eventStart, log)

	switch productInfo.Source {
	case setting.SourceFromHelm:
		err = mongodb.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		go func() {
			var err error
			defer func() {
				if err != nil {
					// 发送删除产品失败消息给用户
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					SendErrorMessage(username, title, requestID, err, log)
					_ = mongodb.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					SendMessage(username, title, content, requestID, log)
				}
			}()

			//卸载helm release资源
			if hc, err := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace); err == nil {
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
			if err1 := updater.DeleteMatchingNamespace(productInfo.Namespace, s, kubeClient); err1 != nil {
				err = e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg + ": " + err1.Error())
				return
			}
		}()
	case setting.SourceFromExternal:
		err = mongodb.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		// 删除workload数据
		tempProduct, err := template.NewProductColl().Find(productName)
		if err != nil {
			log.Errorf("project not found error:%s", err)
		}
		if tempProduct.ProductFeature != nil && tempProduct.ProductFeature.CreateEnvType == setting.SourceFromExternal {
			workloadStat, err := mongodb.NewWorkLoadsStatColl().Find(productInfo.ClusterID, productInfo.Namespace)
			if err != nil {
				log.Errorf("workflowStat not found error:%s", err)
			}
			if workloadStat != nil {
				workloadStat.Workloads = filterWorkloadsByEnv(workloadStat.Workloads, productInfo.EnvName)
				if err := mongodb.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat); err != nil {
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

	default:
		go func() {
			var err error
			defer func() {
				if err != nil {
					// 发送删除产品失败消息给用户
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					SendErrorMessage(username, title, requestID, err, log)
					_ = mongodb.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					SendMessage(username, title, content, requestID, log)
				}
			}()

			err = DeleteResourcesAsync(productInfo.Namespace, labels.Set{setting.ProductLabel: productName}.AsSelector(), kubeClient, log)
			if err != nil {
				err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
				return
			}

			err = DeleteClusterResourceAsync(labels.Set{setting.ProductLabel: productName, setting.EnvNameLabel: envName}.AsSelector(), kubeClient, log)

			if err != nil {
				err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
				return
			}

			s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
			if err1 := updater.DeleteMatchingNamespace(productInfo.Namespace, s, kubeClient); err1 != nil {
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

			err = mongodb.NewProductColl().Delete(envName, productName)
			if err != nil {
				log.Errorf("Product.Delete error: %v", err)
			}
		}()
	}
	return nil
}

func filterWorkloadsByEnv(exist []models.Workload, env string) []models.Workload {
	result := make([]models.Workload, 0)
	for _, v := range exist {
		if v.EnvName != env {
			result = append(result, v)
		}
	}
	return result
}

func DeleteClusterResourceAsync(selector labels.Selector, kubeClient client.Client, log *zap.SugaredLogger) error {
	log.Infof("[%s] delete cluster kube resources", selector)

	errors := new(multierror.Error)
	if err := updater.DeleteClusterRoles(selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteClusterRole error: %v", err))
	}

	if err := updater.DeleteClusterRoleBindings(selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteClusterRoleBinding error: %v", err))
	}

	return errors.ErrorOrNil()
}

// 根据namespace和selector删除所有资源
func DeleteResourcesAsync(namespace string, selector labels.Selector, kubeClient client.Client, log *zap.SugaredLogger) error {
	log.Infof("[%s][%s] delete kube resources", namespace, selector)

	errors := new(multierror.Error)

	if err := updater.DeleteDeployments(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteDeployments error: %v", err))
	}

	// could have replicas created by deployment
	if err := updater.DeleteReplicaSets(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteReplicaSets error: %v", err))
	}

	if err := updater.DeleteStatefulSets(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteStatefulSets error: %v", err))
	}

	if err := updater.DeleteJobs(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteJobs error: %v", err))
	}

	if err := updater.DeleteServices(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteServices error: %v", err))
	}

	if err := updater.DeleteIngresses(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteIngresses error: %v", err))
	}

	if err := updater.DeleteSecrets(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteSecrets error: %v", err))
	}

	if err := updater.DeleteConfigMaps(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteConfigMaps error: %v", err))
	}

	if err := updater.DeletePersistentVolumeClaims(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeletePersistentVolumeClaim error: %v", err))
	}

	if err := updater.DeletePersistentVolumes(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeletePersistentVolume error: %v", err))
	}

	if err := updater.DeleteServiceAccounts(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteServiceAccounts error: %v", err))
	}

	if err := updater.DeleteCronJobs(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteCronJobs error: %v", err))
	}

	if err := updater.DeleteRoleBindings(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteRoleBinding error: %v", err))
	}

	if err := updater.DeleteRoles(namespace, selector, kubeClient); err != nil {
		log.Error(err)
		errors = multierror.Append(errors, fmt.Errorf("kubeCli.DeleteRole error: %v", err))
	}

	return errors.ErrorOrNil()
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
