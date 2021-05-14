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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func ListKubeEvents(env string, productName string, name string, rtype string, log *xlog.Logger) ([]*resource.Event, error) {
	res := make([]*resource.Event, 0)
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: env,
	})
	if err != nil {
		return res, err
	}

	// cached client does not support label/field selector which has more than one kv, so we need a apiReader
	// here to read from API Server directly.
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterId)
	if err != nil {
		return res, err
	}

	selector := fields.Set{"involvedObject.name": name, "involvedObject.kind": rtype}.AsSelector()
	events, err := getter.ListEvents(product.Namespace, selector, kubeClient)

	if err != nil {
		log.Errorf("failed to list kube events %s/%s/%s, err: %s", product.Namespace, rtype, name, err)
		return res, e.ErrListPodEvents.AddErr(err)
	}

	for _, evt := range events {
		res = append(res, wrapper.Event(evt).Resource())
	}

	return res, err
}

func ListPodEvents(envName, productName, podName string, log *xlog.Logger) ([]*resource.Event, error) {
	res := make([]*resource.Event, 0)
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return res, err
	}

	// cached client does not support label/field selector which has more than one kv, so we need a apiReader
	// here to read from API Server directly.
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterId)
	if err != nil {
		return res, err
	}

	selector := fields.Set{"involvedObject.name": podName, "involvedObject.kind": setting.Pod}.AsSelector()
	events, err := getter.ListEvents(product.Namespace, selector, kubeClient)
	if err != nil {
		log.Error(err)
		return res, err
	}

	for _, evt := range events {
		res = append(res, wrapper.Event(evt).Resource())
	}

	return res, nil
}

// ListAvailableNamespaces lists available namespaces which are not used by any environments.
func ListAvailableNamespaces(clusterID string, log *xlog.Logger) ([]*resource.Namespace, error) {
	resp := make([]*resource.Namespace, 0)
	kubeClient, err := kube.GetKubeClient(clusterID)
	if err != nil {
		log.Errorf("ListNamespaces clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	namespaces, err := getter.ListNamespaces(kubeClient)
	if err != nil {
		log.Errorf("ListNamespaces err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, nil
		}
		return resp, err
	}

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("Collection.Product.List error: %v", err)
		return resp, err
	}

	productNamespaces := sets.NewString()
	for _, product := range products {
		productNamespaces.Insert(product.Namespace)
	}

	for _, namespace := range namespaces {
		if value, IsExist := namespace.Labels[setting.EnvCreatedBy]; IsExist {
			if value == setting.EnvCreator {
				continue
			}
		}

		if productNamespaces.Has(namespace.Name) {
			continue
		}

		resp = append(resp, wrapper.Namespace(namespace).Resource())
	}

	return resp, nil
}

func ListServicePods(productName, envName string, serviceName string, log *xlog.Logger) ([]*resource.Pod, error) {
	res := make([]*resource.Pod, 0)

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return res, e.ErrListServicePod.AddErr(err)
	}
	kubeClient, err := kube.GetKubeClient(product.ClusterId)
	if err != nil {
		return res, e.ErrListServicePod.AddErr(err)
	}

	selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
	pods, err := getter.ListPods(product.Namespace, selector, kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] ListServicePods %s error: %v", product.Namespace, selector, err)
		log.Error(errMsg)
		return res, e.ErrListServicePod.AddDesc(errMsg)
	}

	for _, pod := range pods {
		res = append(res, wrapper.Pod(pod).Resource())
	}
	return res, nil
}

func DeletePod(envName, productName, podName string, log *xlog.Logger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}
	kubeClient, err := kube.GetKubeClient(product.ClusterId)
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}

	namespace := product.Namespace
	err = updater.DeletePod(namespace, podName, kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] delete pod %s error: %v", namespace, podName, err)
		log.Error(errMsg)
		return e.ErrDeletePod.AddDesc(errMsg)
	}
	return nil
}
