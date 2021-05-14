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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/patcher"
	"github.com/koderover/zadig/lib/tool/kube/util"
)

func patchObject(obj client.Object, patchBytes []byte, cl client.Client) error {
	return cl.Patch(context.TODO(), obj, client.RawPatch(types.StrategicMergePatchType, patchBytes))
}

func updateObject(obj client.Object, cl client.Client) error {
	// always add the last-applied-configuration annotation
	err := util.CreateApplyAnnotation(obj)
	if err != nil {
		return err
	}
	return cl.Update(context.TODO(), obj)
}

func deleteObject(obj client.Object, cl client.Client, opts ...client.DeleteOption) error {
	return cl.Delete(context.TODO(), obj, opts...)
}

func deleteObjects(obj client.Object, cl client.Client, opts ...client.DeleteAllOfOption) error {
	return cl.DeleteAllOf(context.TODO(), obj, opts...)
}

func deleteObjectWithDefaultOptions(obj client.Object, cl client.Client) error {
	deletePolicy := metav1.DeletePropagationForeground
	return deleteObject(obj, cl, &client.DeleteOptions{PropagationPolicy: &deletePolicy})
}

func deleteObjectsWithDefaultOptions(ns string, selector labels.Selector, obj client.Object, cl client.Client) error {
	deletePolicy := metav1.DeletePropagationForeground

	delOpt := &client.DeleteAllOfOptions{
		DeleteOptions: client.DeleteOptions{PropagationPolicy: &deletePolicy},
		ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: ns},
	}

	return deleteObjects(obj, cl, delOpt)
}

func createObject(obj client.Object, cl client.Client) error {
	// always add the last-applied-configuration annotation
	err := util.CreateApplyAnnotation(obj)
	if err != nil {
		return err
	}
	return cl.Create(context.TODO(), obj)
}

func updateOrCreateObject(obj client.Object, cl client.Client) error {
	err := updateObject(obj, cl)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return createObject(obj, cl)
		}

		return err
	}

	return nil
}

func createOrPatchObject(modified client.Object, cl client.Client) error {
	c := modified.DeepCopyObject()
	current := c.(client.Object)
	found, err := getter.GetResourceInCache(modified.GetNamespace(), modified.GetName(), current, cl)
	if err != nil {
		return err
	} else if !found {
		return createObject(modified, cl)
	}

	patchBytes, patchType, err := patcher.GeneratePatchBytes(current, modified)
	if err != nil {
		return err
	}

	if len(patchBytes) == 0 || patchType == "" || string(patchBytes) == "{}" {
		return nil
	}

	return patchObjectWithType(current, patchBytes, patchType, cl)
}

func patchObjectWithType(obj client.Object, patchBytes []byte, patchType types.PatchType, cl client.Client) error {
	return cl.Patch(context.TODO(), obj, client.RawPatch(patchType, patchBytes))
}

// TODO: LOU: improve it
// deleteObjectAndWait delete the object and wait till it is gone.
func deleteObjectAndWait(obj client.Object, cl client.Client) error {
	err := deleteObjectWithDefaultOptions(obj, cl)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
		found, err := getter.GetResourceInCache(obj.GetNamespace(), obj.GetName(), obj, cl)
		if err != nil {
			return false, err
		}

		return !found, nil
	})
}

// TODO: LOU: improve it
// deleteObjectsAndWait deletes the objects and wait till all of them are gone.
func deleteObjectsAndWait(ns string, selector labels.Selector, obj client.Object, gvk schema.GroupVersionKind, cl client.Client) error {
	err := deleteObjectsWithDefaultOptions(ns, selector, obj, cl)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
		us, err := getter.ListUnstructuredResourceInCache(ns, selector, nil, gvk, cl)
		if err != nil {
			return false, err
		}

		return len(us) == 0, nil
	})
}
