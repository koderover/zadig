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
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	timeout = 60
)

func DeleteProduct(username, envName, productName string, log *xlog.Logger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := repo.NewProductColl().Find(&repo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddDesc("not found")
	}

	kubeClient, err := kube.GetKubeClient(productInfo.ClusterId)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	// 设置产品状态
	err = repo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusDeleting)
	if err != nil {
		log.Errorf("[%s][%s] update product status error: %v", username, productInfo.Namespace, err)
		return e.ErrDeleteEnv.AddDesc("更新集成环境状态失败: " + err.Error())
	}

	log.Infof("[%s] delete product %s", username, productInfo.Namespace)
	LogProductStats(username, setting.DeleteProductEvent, productName, eventStart, log)

	poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())

	switch productInfo.Source {
	default:
		go func() {
			var err error
			defer func() {
				if err != nil {
					// 发送删除产品失败消息给用户
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					SendErrorMessage(username, title, err, log)
					_ = repo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					SendMessage(username, title, content, log)
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

			err = repo.NewProductColl().Delete(envName, productName)
			if err != nil {
				log.Errorf("Product.Delete error: %v", err)
			}

			_, err = poetryClient.DeleteEnvRolePermission(productName, envName, log)
			if err != nil {
				log.Errorf("DeleteEnvRole error: %v", err)
			}
		}()
	}

	return nil
}

func DeleteClusterResourceAsync(selector labels.Selector, kubeClient client.Client, log *xlog.Logger) error {
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
func DeleteResourcesAsync(namespace string, selector labels.Selector, kubeClient client.Client, log *xlog.Logger) error {
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

func GetProductEnvNamespace(envName, productName string) string {
	product := &commonmodels.Product{EnvName: envName, ProductName: productName}
	return product.GetNamespace()
}
