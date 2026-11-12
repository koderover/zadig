/*
Copyright 2026 The KodeRover Authors.

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

package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"helm.sh/helm/v3/pkg/releaseutil"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	resourceDeletionTimeout      = time.Second
	resourceDeletionPollInterval = 100 * time.Millisecond
	crdRegistrationTimeout       = 60 * time.Second
	crdRegistrationPollInterval  = time.Second
)

func resourceGVKNKey(u *unstructured.Unstructured) string {
	return fmt.Sprintf("%s-%s", u.GroupVersionKind(), u.GetName())
}

func resourceKindNameKey(u *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())
}

func isCRD(u *unstructured.Unstructured) bool {
	return u.GroupVersionKind().Group == apiextensionsv1.GroupName && u.GetKind() == "CustomResourceDefinition"
}

func orderedResourcesToUpdate(resources []*unstructured.Unstructured, pending map[string]*Resource) []*unstructured.Unstructured {
	ordered := make([]*unstructured.Unstructured, 0, len(pending))
	for _, u := range resources {
		key := resourceGVKNKey(u)
		if r, ok := pending[key]; ok && r.Unstructured == u {
			ordered = append(ordered, u)
		}
	}
	priorities := make(map[string]int, len(releaseutil.InstallOrder))
	for i, kind := range releaseutil.InstallOrder {
		priorities[kind] = i
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].GetKind(), ordered[j].GetKind()
		ai, aKnown := priorities[a]
		bi, bKnown := priorities[b]
		if aKnown != bKnown {
			return aKnown
		}
		if aKnown {
			return ai < bi
		}
		return a < b
	})
	return ordered
}

func deletionPriority(u *unstructured.Unstructured) int {
	if isCRD(u) {
		return len(releaseutil.UninstallOrder)
	}
	// Submit deletion of unknown kinds, including CRs, before their controllers.
	return slices.Index(releaseutil.UninstallOrder, u.GetKind())
}

func orderedResourcesToRemove(current, desired []*unstructured.Unstructured) []*unstructured.Unstructured {
	keep := make(map[string]bool, len(desired))
	for _, u := range desired {
		keep[resourceKindNameKey(u)] = true
	}
	last := make(map[string]*unstructured.Unstructured, len(current))
	for _, u := range current {
		last[resourceKindNameKey(u)] = u
	}
	ordered := make([]*unstructured.Unstructured, 0, len(current))
	for _, u := range current {
		key := resourceKindNameKey(u)
		if !keep[key] && last[key] == u {
			ordered = append(ordered, u)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := deletionPriority(ordered[i]), deletionPriority(ordered[j])
		if a != b {
			return a < b
		}
		return ordered[i].GetKind() < ordered[j].GetKind()
	})
	return ordered
}

func waitForDeletedResources(ctx context.Context, resources []*unstructured.Unstructured, reader client.Reader) error {
	ctx, cancel := context.WithTimeout(ctx, resourceDeletionTimeout)
	defer cancel()
	for _, u := range resources {
		err := wait.PollUntilContextCancel(ctx, resourceDeletionPollInterval, true, func(ctx context.Context) (bool, error) {
			err := reader.Get(ctx, client.ObjectKeyFromObject(u), u.DeepCopy())
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			return fmt.Errorf("failed to wait for deletion of %s/%s: %w", u.GetKind(), u.GetName(), err)
		}
	}
	return nil
}

// CRDs must be registered and discoverable before ordinary resources can use their APIs.
func applyCRDs(ctx context.Context, crds []*unstructured.Unstructured, pending map[string]*Resource, apiClient rest.Interface, apply func(*unstructured.Unstructured) error) error {
	for _, crd := range crds {
		key := resourceGVKNKey(crd)
		if r, ok := pending[key]; !ok || r.Unstructured != crd {
			continue
		}
		crd.SetNamespace("")
		if err := apply(crd); err != nil {
			return fmt.Errorf("failed to apply CRD %s: %w", crd.GetName(), err)
		}
	}
	for _, crd := range crds {
		var definition apiextensionsv1.CustomResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(crd.Object, &definition); err != nil {
			return fmt.Errorf("failed to read CRD %s: %w", crd.GetName(), err)
		}
		if len(definition.Spec.Versions) == 0 {
			version, _, err := unstructured.NestedString(crd.Object, "spec", "version")
			if err != nil {
				return fmt.Errorf("failed to read CRD %s version: %w", crd.GetName(), err)
			}
			if version != "" {
				definition.Spec.Versions = []apiextensionsv1.CustomResourceDefinitionVersion{{Name: version, Served: true}}
			}
		}
		err := wait.PollUntilContextCancel(ctx, crdRegistrationPollInterval, true, func(ctx context.Context) (bool, error) {
			data, err := apiClient.Get().AbsPath("/apis", crd.GetAPIVersion(), "customresourcedefinitions", crd.GetName()).Do(ctx).Raw()
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			var current apiextensionsv1.CustomResourceDefinition
			if err := json.Unmarshal(data, &current); err != nil {
				return false, err
			}
			established := false
			for _, condition := range current.Status.Conditions {
				if condition.Type == apiextensionsv1.NamesAccepted && condition.Status == apiextensionsv1.ConditionFalse {
					return false, fmt.Errorf("names rejected: %s: %s", condition.Reason, condition.Message)
				}
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					established = true
				}
			}
			if !established {
				return false, nil
			}
			for _, version := range definition.Spec.Versions {
				if !version.Served {
					continue
				}
				data, err := apiClient.Get().AbsPath("/apis", definition.Spec.Group, version.Name).Do(ctx).Raw()
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				var resources metav1.APIResourceList
				if err := json.Unmarshal(data, &resources); err != nil {
					return false, err
				}
				found := false
				for _, resource := range resources.APIResources {
					if resource.Name == definition.Spec.Names.Plural && resource.Kind == definition.Spec.Names.Kind {
						found = true
						break
					}
				}
				if !found {
					return false, nil
				}
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("failed to wait for CRD %s registration: %w", crd.GetName(), err)
		}
	}
	return nil
}
