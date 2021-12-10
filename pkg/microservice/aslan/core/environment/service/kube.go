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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/kube/util"
)

type serviceInfo struct {
	Name           string    `json:"name"`
	ModifiedBy     string    `json:"modifiedBy"`
	LastUpdateTime time.Time `json:"-"`
}

func ListKubeEvents(env string, productName string, name string, rtype string, log *zap.SugaredLogger) ([]*resource.Event, error) {
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
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterID)
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

func ListPodEvents(envName, productName, podName string, log *zap.SugaredLogger) ([]*resource.Event, error) {
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
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterID)
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

// ListAvailableNamespaces lists available namespaces created by non-koderover
func ListAvailableNamespaces(clusterID string, log *zap.SugaredLogger) ([]*resource.Namespace, error) {
	resp := make([]*resource.Namespace, 0)
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("ListNamespaces clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	namespaces, err := getter.ListNamespaces(kubeClient)
	if err != nil {
		log.Errorf("ListNamespaces err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}

	for _, namespace := range namespaces {
		if value, IsExist := namespace.Labels[setting.EnvCreatedBy]; IsExist {
			if value == setting.EnvCreator {
				continue
			}
		}

		resp = append(resp, wrapper.Namespace(namespace).Resource())
	}

	return resp, nil
}

func ListServicePods(productName, envName string, serviceName string, log *zap.SugaredLogger) ([]*resource.Pod, error) {
	res := make([]*resource.Pod, 0)

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return res, e.ErrListServicePod.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
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

func DeletePod(envName, productName, podName string, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
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

// getServiceFromObjectMetaList returns a set of services which are modified since last update.
// Input is all modified objects, if there are more than one objects are changed, only the last one is taken into account.
func getModifiedServiceFromObjectMetaList(oms []metav1.Object) []*serviceInfo {
	sis := make(map[string]*serviceInfo)
	for _, om := range oms {
		si := getModifiedServiceFromObjectMeta(om)
		if si.ModifiedBy == "" || si.Name == "" {
			continue
		}
		if old, ok := sis[si.Name]; ok {
			if !si.LastUpdateTime.After(old.LastUpdateTime) {
				continue
			}
		}
		sis[si.Name] = si
	}

	var res []*serviceInfo
	for _, si := range sis {
		res = append(res, si)
	}

	return res
}

func getModifiedServiceFromObjectMeta(om metav1.Object) *serviceInfo {
	ls := om.GetLabels()
	as := om.GetAnnotations()
	t, _ := util.ParseTime(as[setting.LastUpdateTimeAnnotation])
	return &serviceInfo{
		Name:           ls[setting.ServiceLabel],
		ModifiedBy:     as[setting.ModifiedByAnnotation],
		LastUpdateTime: t,
	}
}

func ListAvailableNodes(clusterID string, log *zap.SugaredLogger) (*NodeResp, error) {
	resp := new(NodeResp)
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("ListAvailableNodes clusterID:%s err:%s", clusterID, err)
		return resp, err
	}

	nodes, err := getter.ListNodes(kubeClient)
	if err != nil {
		log.Errorf("ListNodes err:%s", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}

	nodeInfos := make([]*resource.Node, 0)
	labels := sets.NewString()
	for _, node := range nodes {
		nodeResource := &resource.Node{
			Ready:  nodeReady(node),
			Labels: nodeLabel(node),
			IP:     node.Name,
		}
		nodeInfos = append(nodeInfos, nodeResource)
		labels.Insert(nodeResource.Labels...)
	}
	resp.Nodes = nodeInfos
	resp.Labels = labels.List()
	return resp, nil
}

// Ready indicates that the node is ready for traffic.
func nodeReady(node *corev1.Node) bool {
	cs := node.Status.Conditions
	for _, c := range cs {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func nodeLabel(node *corev1.Node) []string {
	labels := make([]string, 0, len(node.Labels))
	labelM := node.Labels
	for key, value := range labelM {
		labels = append(labels, fmt.Sprintf("%s:%s", key, value))
	}
	return labels
}
