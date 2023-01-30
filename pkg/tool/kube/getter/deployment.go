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

package getter

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var DeploymentGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "Deployment",
	Version: "v1",
}

func GetDeployment(ns, name string, cl client.Client) (*appsv1.Deployment, bool, error) {
	g := &appsv1.Deployment{}

	found, err := GetResourceInCache(ns, name, g, cl)
	if err != nil || !found {
		g = nil
	}
	setDeploymentGVK(g)
	return g, found, err
}

func ListDeployments(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.Deployment, error) {
	ss := &appsv1.DeploymentList{}
	err := ListResourceInCache(ns, selector, nil, ss, cl)
	if err != nil {
		return nil, err
	}

	var res []*appsv1.Deployment
	for i := range ss.Items {
		setDeploymentGVK(&ss.Items[i])
		res = append(res, &ss.Items[i])
	}
	return res, err
}

func ListDeploymentsWithCache(selector labels.Selector, lister informers.SharedInformerFactory) ([]*appsv1.Deployment, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Apps().V1().Deployments().Lister().List(selector)
}

func GetDeploymentByNameWithCache(name, namespace string, lister informers.SharedInformerFactory) (*appsv1.Deployment, error) {
	return lister.Apps().V1().Deployments().Lister().Deployments(namespace).Get(name)
}

func ListDeploymentsYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	return ListResourceYamlInCache(ns, selector, nil, DeploymentGVK, cl)
}

func GetDeploymentYaml(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCache(ns, name, DeploymentGVK, cl)
}

func GetDeploymentYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCacheFormat(ns, name, DeploymentGVK, cl)
}

func setDeploymentGVK(deployment *appsv1.Deployment) {
	if deployment == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	}
	deployment.SetGroupVersionKind(gvk)
}
