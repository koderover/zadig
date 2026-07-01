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
	if resources[0].APIVersion != "apps/v1" || resources[0].Kind != "Deployment" || resources[0].Name != "demo" {
		t.Fatalf("unexpected first resource: %+v", resources[0])
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
	if resources[0].Kind != "Service" || resources[1].Kind != "Ingress" {
		t.Fatalf("unexpected filtered resources: %+v", resources)
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
