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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestManifestToUnstructuredOrder(t *testing.T) {
	var yaml strings.Builder
	yaml.WriteString("# comments only\n---\n")
	for i := 0; i < 14; i++ {
		fmt.Fprintf(&yaml, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config-%d\n---\n", i)
	}
	yaml.WriteString("# trailing comment\n")
	resources, lookup, err := ManifestToUnstructured(yaml.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 14 || len(lookup) != 14 {
		t.Fatalf("resource counts: %d, %d", len(resources), len(lookup))
	}
	for i, resource := range resources {
		if resource.GetName() != fmt.Sprintf("config-%d", i) {
			t.Fatalf("document %d: %s", i, resource.GetName())
		}
	}
	if resources, _, err := ManifestToUnstructured(""); err != nil || len(resources) != 0 {
		t.Fatalf("empty manifest: %v, %v", resources, err)
	}
	if _, _, err := ManifestToUnstructured("apiVersion: [invalid"); err == nil {
		t.Fatal("expected invalid YAML to fail")
	}
}

func TestOrderedResourcesToUpdate(t *testing.T) {
	kinds := []string{"Widget", "Deployment", "ConfigMap", "Ingress", "ConfigMap", "Secret", "PodDisruptionBudget", "Service", "Role", "PersistentVolumeClaim", "Namespace", "Zebra", "Widget", "CustomResourceDefinition"}
	resources := make([]*unstructured.Unstructured, 0, len(kinds))
	pending := make(map[string]*Resource)
	for i, kind := range kinds {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: kind})
		u.SetName(fmt.Sprintf("resource-%d", i))
		resources = append(resources, u)
		pending[resourceGVKNKey(u)] = &Resource{Unstructured: u}
	}
	assertOrder := func(want []int) {
		t.Helper()
		got := orderedResourcesToUpdate(resources, pending)
		var names []string
		var expected []string
		for _, u := range got {
			names = append(names, u.GetName())
		}
		for _, i := range want {
			expected = append(expected, fmt.Sprintf("resource-%d", i))
		}
		if !reflect.DeepEqual(names, expected) {
			t.Fatalf("got %v, want %v", names, expected)
		}
	}
	assertOrder([]int{10, 6, 5, 2, 4, 9, 13, 8, 7, 1, 3, 0, 12, 11})
	// Unchanged resources are removed from the lookup, without reordering pending resources.
	delete(pending, resourceGVKNKey(resources[2]))
	assertOrder([]int{10, 6, 5, 4, 9, 13, 8, 7, 1, 3, 0, 12, 11})
	// Keep the existing last-document-wins behavior for duplicate resource identities.
	duplicate := resources[4].DeepCopy()
	resources = append(resources, duplicate)
	pending[resourceGVKNKey(duplicate)] = &Resource{Unstructured: duplicate}
	assertOrder([]int{10, 6, 5, 4, 9, 13, 8, 7, 1, 3, 0, 12, 11})
}

func TestParsedResourceKeysMatchDeployment(t *testing.T) {
	resources, pending, err := ManifestToUnstructured(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
---
apiVersion: example.com/v1
kind: Widget
metadata:
  name: example
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
`)
	if err != nil {
		t.Fatal(err)
	}
	ordered := orderedResourcesToUpdate(resources, pending)
	if len(ordered) != 3 || ordered[0] != resources[2] || ordered[1] != resources[0] || ordered[2] != resources[1] {
		t.Fatalf("parsed resources were skipped or reordered incorrectly: %v", ordered)
	}
	applyErr := errors.New("reached CRD apply")
	err = applyCRDs(context.Background(), resources[:1], pending, &restfake.RESTClient{Err: errors.New("unexpected registration request")}, func(u *unstructured.Unstructured) error {
		if u != resources[0] {
			t.Fatalf("unexpected CRD: %v", u)
		}
		return applyErr
	})
	if !errors.Is(err, applyErr) {
		t.Fatalf("CRD from parser was not passed to apply: %v", err)
	}
}

func TestApplyCRDs(t *testing.T) {
	definition := apiextensionsv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group:    "example.com",
			Names:    apiextensionsv1.CustomResourceDefinitionNames{Plural: "widgets", Kind: "Widget"},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true}, {Name: "v2", Served: false}},
		},
	}
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&definition)
	if err != nil {
		t.Fatal(err)
	}
	crd := &unstructured.Unstructured{Object: object}
	if !isCRD(crd) || isCRD(&unstructured.Unstructured{}) {
		t.Fatal("incorrect CRD classification")
	}
	for _, scenario := range []string{"registered", "multiple CRDs", "legacy CRD", "registration delayed", "discovery delayed", "API not registered", "unchanged", "apply failed", "names rejected", "forbidden", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			applied, checked, discovered := 0, 0, 0
			u := crd.DeepCopy()
			u.SetNamespace("must-be-cleared")
			if scenario == "legacy CRD" {
				u.SetAPIVersion("apiextensions.k8s.io/v1beta1")
				unstructured.RemoveNestedField(u.Object, "spec", "versions")
				if err := unstructured.SetNestedField(u.Object, "v1", "spec", "version"); err != nil {
					t.Fatal(err)
				}
			}
			crds := []*unstructured.Unstructured{u}
			if scenario == "multiple CRDs" {
				second := u.DeepCopy()
				second.SetName("gadgets.example.com")
				if err := unstructured.SetNestedMap(second.Object, map[string]interface{}{"plural": "gadgets", "kind": "Gadget"}, "spec", "names"); err != nil {
					t.Fatal(err)
				}
				crds = append(crds, second)
			}
			pending := make(map[string]*Resource)
			for _, crd := range crds {
				pending[resourceGVKNKey(crd)] = &Resource{Unstructured: crd}
			}
			if scenario == "unchanged" {
				pending = nil
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/apis/apiextensions.k8s.io/v1/customresourcedefinitions/widgets.example.com",
					"/apis/apiextensions.k8s.io/v1/customresourcedefinitions/gadgets.example.com",
					"/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/widgets.example.com":
					checked++
					if scenario != "unchanged" && applied != len(crds) {
						t.Error("registration queried before apply")
					}
					if scenario == "forbidden" {
						w.WriteHeader(http.StatusForbidden)
						json.NewEncoder(w).Encode(metav1.Status{Status: "Failure", Reason: metav1.StatusReasonForbidden, Code: http.StatusForbidden})
						return
					}
					current := definition.DeepCopy()
					if strings.HasSuffix(r.URL.Path, "/gadgets.example.com") {
						current.Name = "gadgets.example.com"
						current.Spec.Names = apiextensionsv1.CustomResourceDefinitionNames{Plural: "gadgets", Kind: "Gadget"}
					}
					current.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue}}
					if scenario == "timeout" || scenario == "registration delayed" && checked == 1 {
						current.Status.Conditions = nil
					}
					if scenario == "names rejected" {
						current.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionFalse, Reason: "PluralConflict"}}
					}
					json.NewEncoder(w).Encode(current)
				case "/apis/example.com/v1":
					discovered++
					if scenario == "API not registered" && discovered == 1 {
						w.WriteHeader(http.StatusNotFound)
						json.NewEncoder(w).Encode(metav1.Status{Status: "Failure", Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound})
						return
					}
					resources := &metav1.APIResourceList{GroupVersion: "example.com/v1", APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget"}, {Name: "gadgets", Kind: "Gadget"}}}
					if scenario == "discovery delayed" && discovered == 1 {
						resources.APIResources = nil
					}
					json.NewEncoder(w).Encode(resources)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			client := &restfake.RESTClient{
				NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				Client: restfake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, r)
					return response.Result(), nil
				}),
			}
			timeout := 5 * time.Second
			if scenario == "timeout" {
				timeout = 50 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			applyErr := errors.New("apply failed")
			err := applyCRDs(ctx, crds, pending, client, func(u *unstructured.Unstructured) error {
				applied++
				if u.GetNamespace() != "" {
					t.Error("CRD must have cluster scope")
				}
				if scenario == "apply failed" {
					return applyErr
				}
				return nil
			})
			wantError := scenario == "apply failed" || scenario == "names rejected" || scenario == "forbidden" || scenario == "timeout"
			if (err != nil) != wantError {
				t.Fatalf("unexpected result: %v", err)
			}
			if wantError && !strings.Contains(err.Error(), definition.Name) {
				t.Fatalf("missing CRD name: %v", err)
			}
			if scenario == "apply failed" && (!errors.Is(err, applyErr) || checked != 0) {
				t.Fatalf("apply failure did not stop registration: %v, checks=%d", err, checked)
			}
			if scenario == "unchanged" && (applied != 0 || checked == 0 || discovered == 0) {
				t.Fatal("unchanged CRD must skip apply and still wait for registration")
			}
			if scenario == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected timeout: %v", err)
			}
			if scenario == "registration delayed" && checked < 2 || (scenario == "discovery delayed" || scenario == "API not registered") && discovered < 2 {
				t.Fatal("did not wait for registration/discovery")
			}
		})
	}
}

func TestRemoveResources(t *testing.T) {
	manifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: operator
---
apiVersion: v1
kind: Secret
metadata:
  name: credentials
---
apiVersion: example.com/v1
kind: Widget
metadata:
  name: first
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: retained-config
---
apiVersion: example.com/v1
kind: ClusterWidget
metadata:
  name: cluster-widget
---
apiVersion: example.com/v1
kind: Widget
metadata:
  name: second
---
apiVersion: v1
kind: Namespace
metadata:
  name: target
---
apiVersion: v1
kind: Service
metadata:
  name: entry
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: budget
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: first-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second-config
`
	resources, _, err := ManifestToUnstructured(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper(nil)
	for _, u := range resources {
		scope := meta.RESTScopeNamespace
		if isCRD(u) || u.GetKind() == "ClusterWidget" || u.GetKind() == "Namespace" {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(u.GroupVersionKind(), scope)
	}
	for _, scenario := range []string{"ordered", "wait", "already absent", "delete failed", "cleanup failed", "wait timeout"} {
		t.Run(scenario, func(t *testing.T) {
			var deleted, checked []string
			cl := clientfake.NewClientBuilder().WithRESTMapper(mapper).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					deleted = append(deleted, obj.GetName())
					wantNamespace := "target"
					if obj.GetObjectKind().GroupVersionKind().Kind == "ClusterWidget" || obj.GetObjectKind().GroupVersionKind().Kind == "Namespace" || isCRD(obj.(*unstructured.Unstructured)) {
						wantNamespace = ""
					}
					if obj.GetNamespace() != wantNamespace {
						t.Errorf("namespace for %s: got %q, want %q", obj.GetName(), obj.GetNamespace(), wantNamespace)
					}
					options := &client.DeleteOptions{}
					options.ApplyOptions(opts)
					if options.PropagationPolicy == nil || *options.PropagationPolicy != metav1.DeletePropagationForeground {
						t.Error("expected foreground deletion")
					}
					if scenario == "delete failed" && obj.GetName() == "first" {
						return errors.New("delete forbidden")
					}
					if scenario == "already absent" {
						return apierrors.NewNotFound(schema.GroupResource{Resource: "widgets"}, obj.GetName())
					}
					return nil
				},
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					checked = append(checked, key.Name)
					if scenario == "cleanup failed" {
						return errors.New("cannot read CR")
					}
					if scenario == "wait timeout" && key.Name == "cluster-widget" {
						return nil
					}
					return apierrors.NewNotFound(schema.GroupResource{Resource: "widgets"}, key.Name)
				},
			}).Build()
			waitForDelete := scenario == "wait" || scenario == "cleanup failed" || scenario == "wait timeout"
			err := removeResources(resources, []*unstructured.Unstructured{resources[4]}, "target", waitForDelete, cl, nil, zap.NewNop().Sugar())
			want := []string{"cluster-widget", "first", "second", "entry", "operator", "first-config", "second-config", "credentials", "budget", "target", "widgets.example.com"}
			wantError := scenario == "delete failed"
			if (err != nil) != wantError {
				t.Fatalf("unexpected result: %v", err)
			}
			if !reflect.DeepEqual(deleted, want) {
				t.Fatalf("deleted %v, want %v", deleted, want)
			}
			if !waitForDelete && len(checked) != 0 {
				t.Fatalf("must not wait unless requested, got %v", checked)
			}
			if (scenario == "wait" || scenario == "cleanup failed") && !reflect.DeepEqual(checked, want) {
				t.Fatalf("expected all resources to be checked, got %v", checked)
			}
			if scenario == "wait timeout" && (len(checked) == 0 || checked[len(checked)-1] != want[len(want)-1]) {
				t.Fatalf("must continue deletion after timeout, checked %v", checked)
			}
			if scenario == "already absent" && len(checked) != 0 {
				t.Fatalf("absent resources should not be waited on: %v", checked)
			}
			for _, u := range resources {
				if u.GetNamespace() != "" {
					t.Fatal("delete must not mutate the original resource list")
				}
			}
		})
	}
}

func TestWaitForDeletedResourcesTimeout(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"})
	u.SetName("stuck-finalizer")
	cl := clientfake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return nil
		},
	}).Build()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := waitForDeletedResources(ctx, []*unstructured.Unstructured{u}, cl)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), u.GetName()) {
		t.Fatalf("expected timeout identifying stuck CR, got %v", err)
	}
}

func TestWaitForDeletedResourcesDetectsDeletionWithinWindow(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"})
	u.SetName("deleted-after-150ms")
	checks := 0
	var deletedAt time.Time
	cl := clientfake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			checks++
			if time.Now().Before(deletedAt) {
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{Resource: "widgets"}, key.Name)
		},
	}).Build()
	deletedAt = time.Now().Add(150 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := waitForDeletedResources(ctx, []*unstructured.Unstructured{u}, cl); err != nil {
		t.Fatalf("deletion inside the wait window was not detected: %v", err)
	}
	if checks < 2 {
		t.Fatalf("expected repeated checks before timeout, got %d", checks)
	}
}
