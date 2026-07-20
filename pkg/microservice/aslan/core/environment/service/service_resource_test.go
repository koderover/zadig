package service

import (
	"strings"
	"testing"
)

const testServiceResourceYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
---
apiVersion: v1
kind: Service
metadata:
  name: demo
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
`

func TestParseK8sServiceResources(t *testing.T) {
	resources, err := parseK8sServiceResources(testServiceResourceYaml)
	if err != nil {
		t.Fatalf("parseK8sServiceResources() error = %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}
	expected := map[string]struct{}{
		"apps/v1/Deployment/demo":           {},
		"v1/Service/demo":                   {},
		"networking.k8s.io/v1/Ingress/demo": {},
	}
	if got := resourceSet(resources); len(got) != len(expected) {
		t.Fatalf("unexpected parsed resources: %+v", resources)
	} else {
		for key := range expected {
			if _, ok := got[key]; !ok {
				t.Fatalf("missing parsed resource %s: %+v", key, resources)
			}
		}
	}
}

func TestFilterSelectedServiceResourceYaml(t *testing.T) {
	filtered, err := filterSelectedServiceResourceYaml(testServiceResourceYaml, []*K8sServiceResource{
		{APIVersion: "v1", Kind: "Service", Name: "demo"},
		{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "demo"},
	})
	if err != nil {
		t.Fatalf("filterSelectedServiceResourceYaml() error = %v", err)
	}

	resources, err := parseK8sServiceResources(filtered)
	if err != nil {
		t.Fatalf("parse filtered yaml error = %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	expected := map[string]struct{}{
		"v1/Service/demo":                   {},
		"networking.k8s.io/v1/Ingress/demo": {},
	}
	if got := resourceSet(resources); len(got) != len(expected) {
		t.Fatalf("unexpected filtered resources: %+v", resources)
	} else {
		for key := range expected {
			if _, ok := got[key]; !ok {
				t.Fatalf("missing filtered resource %s: %+v", key, resources)
			}
		}
	}
	if strings.Contains(filtered, "kind: Deployment") {
		t.Fatalf("filtered yaml should not contain Deployment: %s", filtered)
	}
}

func TestFilterSelectedServiceResourceYamlEmptySelection(t *testing.T) {
	filtered, err := filterSelectedServiceResourceYaml(testServiceResourceYaml, nil)
	if err != nil {
		t.Fatalf("filterSelectedServiceResourceYaml() error = %v", err)
	}
	if filtered != "" {
		t.Fatalf("expected empty yaml, got %q", filtered)
	}
}

func TestFilterSelectedServiceResourceYamlMissingResource(t *testing.T) {
	_, err := filterSelectedServiceResourceYaml(testServiceResourceYaml, []*K8sServiceResource{
		{APIVersion: "v1", Kind: "ConfigMap", Name: "missing"},
	})
	if err == nil {
		t.Fatal("expected missing resource error")
	}
}

func resourceSet(resources []*K8sServiceResource) map[string]struct{} {
	ret := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		ret[resource.APIVersion+"/"+resource.Kind+"/"+resource.Name] = struct{}{}
	}
	return ret
}
