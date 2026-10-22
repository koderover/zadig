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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ReplicaSetGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "ReplicaSet",
	Version: "v1",
}

func ListReplicaSets(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.ReplicaSet, error) {
	ss := &appsv1.ReplicaSetList{}
	err := ListResourceInCache(ns, selector, nil, ss, cl)
	if err != nil {
		return nil, err
	}

	var res []*appsv1.ReplicaSet
	for i := range ss.Items {
		setReplicaSetGVK(&ss.Items[i])
		res = append(res, &ss.Items[i])
	}
	return res, err
}

// FindDeploymentReplicaSet finds the owned ReplicaSet matching the current pod
// template, ignoring only pod-template-hash. Like Kubernetes, it chooses the
// oldest matching ReplicaSet, using the name to break creation timestamp ties.
func FindDeploymentReplicaSet(deployment *appsv1.Deployment, replicaSets []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	template := deployment.Spec.Template.DeepCopy()
	delete(template.Labels, appsv1.DefaultDeploymentUniqueLabelKey)

	var matched *appsv1.ReplicaSet
	for _, rs := range replicaSets {
		if !metav1.IsControlledBy(rs, deployment) {
			continue
		}
		rsTemplate := rs.Spec.Template.DeepCopy()
		delete(rsTemplate.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
		if !apiequality.Semantic.DeepEqual(template, rsTemplate) {
			continue
		}
		if matched == nil || rs.CreationTimestamp.Before(&matched.CreationTimestamp) ||
			(rs.CreationTimestamp.Equal(&matched.CreationTimestamp) && rs.Name < matched.Name) {
			matched = rs
		}
	}
	return matched
}

func setReplicaSetGVK(replicaSet *appsv1.ReplicaSet) {
	if replicaSet == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "ReplicaSet",
		Version: "v1",
	}
	replicaSet.SetGroupVersionKind(gvk)
}
