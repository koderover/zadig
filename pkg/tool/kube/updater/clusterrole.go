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

package updater

import (
	"context"

	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/tool/kube/util"
)

func DeleteClusterRoles(selector labels.Selector, clientset *kubernetes.Clientset) error {
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	deletePolicy := metav1.DeletePropagationForeground
	if kubeclient.VersionLessThan122(version) {
		err = clientset.RbacV1beta1().ClusterRoles().DeleteCollection(context.TODO(), metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
	} else {
		err = clientset.RbacV1().ClusterRoles().DeleteCollection(context.TODO(), metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
	}

	return util.IgnoreNotFoundError(err)
}
